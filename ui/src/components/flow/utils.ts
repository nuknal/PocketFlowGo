import dagre from 'dagre'
import { type Node, type Edge, Position, MarkerType } from '@xyflow/react'

const nodeWidth = 220
const nodeHeight = 100

export const getLayoutedElements = (nodes: Node[], edges: Edge[]) => {
  const dagreGraph = new dagre.graphlib.Graph({ compound: true })
  dagreGraph.setDefaultEdgeLabel(() => ({}))

  dagreGraph.setGraph({ rankdir: 'LR', ranksep: 60, nodesep: 60 })

  nodes.forEach((node) => {
    if (node.type === 'group') {
      dagreGraph.setNode(node.id, {
        label: node.id,
        clusterLabelPos: 'top',
        paddingLeft: 20,
        paddingRight: 20,
        paddingTop: 30,
        paddingBottom: 20,
      })
    } else {
      dagreGraph.setNode(node.id, { width: nodeWidth, height: nodeHeight })
    }

    if (node.parentId) {
      dagreGraph.setParent(node.id, node.parentId)
    }
  })

  edges.forEach((edge) => {
    dagreGraph.setEdge(edge.source, edge.target)
  })

  dagre.layout(dagreGraph)

  const newNodes = nodes.map((node) => {
    const nodeWithPosition = dagreGraph.node(node.id)

    // If it's a group node, dagre gives us its calculated size and position
    if (node.type === 'group') {
      return {
        ...node,
        position: {
          x: nodeWithPosition.x - nodeWithPosition.width / 2,
          y: nodeWithPosition.y - nodeWithPosition.height / 2,
        },
        style: {
          ...node.style,
          width: nodeWithPosition.width,
          height: nodeWithPosition.height,
        },
      }
    }

    // For normal nodes
    let position = {
      x: nodeWithPosition.x - nodeWidth / 2,
      y: nodeWithPosition.y - nodeHeight / 2,
    }

    // If it has a parent, convert absolute position to relative position
    if (node.parentId) {
      const parentNode = dagreGraph.node(node.parentId)
      const parentX = parentNode.x - parentNode.width / 2
      const parentY = parentNode.y - parentNode.height / 2

      position = {
        x: position.x - parentX,
        y: position.y - parentY,
      }
    }

    return {
      ...node,
      targetPosition: Position.Left,
      sourcePosition: Position.Right,
      position,
    }
  })

  return { nodes: newNodes, edges }
}

export const expandFlowDefinition = async (
  definitionJson: string,
  isDark: boolean = false
) => {
  let def: any = {}
  try {
    def = JSON.parse(definitionJson)
  } catch (e) {
    return { nodes: [], edges: [] }
  }

  const commonEdgeStyle = {
    type: 'smoothstep',
    markerEnd: {
      type: MarkerType.ArrowClosed,
      color: isDark ? '#94a3b8' : '#333',
    }, // slate-400 : gray-800
    animated: false,
    style: { stroke: isDark ? '#94a3b8' : '#333', strokeWidth: 2 },
  }

  const processDefinition = async (
    currentDef: any,
    prefix: string,
    parentId: string | null = null
  ): Promise<{
    nodes: Node[]
    edges: Edge[]
    entryNodeId: string | null
    exitNodeIds: string[]
  }> => {
    const currentNodes: Node[] = []
    const currentEdges: Edge[] = []
    const rawNodes = currentDef.nodes || {}
    const nodeInfo: Record<string, { entryId: string; exitIds: string[] }> = {}

    // 1. Process Nodes
    for (const key of Object.keys(rawNodes)) {
      const nodeData = rawNodes[key]
      const globalKey = prefix ? `${prefix}_${key}` : key

      if (nodeData.kind === 'subflow' && nodeData.subflow) {
        // Create Group Node for subflow
        const groupNode: Node = {
          id: globalKey,
          data: { label: `Subflow: ${key}`, ...nodeData },
          position: { x: 0, y: 0 },
          type: 'group',
          parentId: parentId || undefined,
          extent: parentId ? 'parent' : undefined,
        }
        currentNodes.push(groupNode)

        // Expand inline subflow with this group as parent
        const subResult = await processDefinition(
          nodeData.subflow,
          globalKey,
          globalKey // Set current group as parent for children
        )
        currentNodes.push(...subResult.nodes)
        currentEdges.push(...subResult.edges)

        nodeInfo[key] = {
          entryId: subResult.entryNodeId || globalKey,
          exitIds:
            subResult.exitNodeIds.length > 0
              ? subResult.exitNodeIds
              : [globalKey],
        }
      } else {
        // Normal node
        currentNodes.push({
          id: globalKey,
          data: { label: key, ...nodeData },
          position: { x: 0, y: 0 },
          type: 'custom',
          parentId: parentId || undefined,
          extent: parentId ? 'parent' : undefined,
        })
        nodeInfo[key] = {
          entryId: globalKey,
          exitIds: [globalKey],
        }
      }
    }

    // 2. Process Edges
    const explicitEdges = currentDef.edges || []
    const nodesWithOutgoingEdges = new Set<string>()

    explicitEdges.forEach((edge: any) => {
      const sourceInfo = nodeInfo[edge.from]
      const targetInfo = nodeInfo[edge.to]

      if (sourceInfo && targetInfo) {
        nodesWithOutgoingEdges.add(edge.from)
        sourceInfo.exitIds.forEach((sourceId) => {
          currentEdges.push({
            id: `${sourceId}-${targetInfo.entryId}-${edge.action || 'default'}`,
            source: sourceId,
            target: targetInfo.entryId,
            label: edge.action,
            ...commonEdgeStyle,
          })
        })
      }
    })

    // 3. Determine Entry and Exit
    const startNodeKey = currentDef.start
    const entryNodeId =
      startNodeKey && nodeInfo[startNodeKey]
        ? nodeInfo[startNodeKey].entryId
        : null

    const exitNodeIds: string[] = []
    Object.keys(rawNodes).forEach((key) => {
      if (!nodesWithOutgoingEdges.has(key)) {
        if (nodeInfo[key]) {
          exitNodeIds.push(...nodeInfo[key].exitIds)
        }
      }
    })

    return {
      nodes: currentNodes,
      edges: currentEdges,
      entryNodeId,
      exitNodeIds,
    }
  }

  const result = await processDefinition(def, '')

  if (result.entryNodeId) {
    const startNode = result.nodes.find((n) => n.id === result.entryNodeId)
    if (startNode) {
      startNode.data.isStart = true
    }
  }

  return getLayoutedElements(result.nodes, result.edges)
}

export const parseFlowDefinition = (definitionJson: string) => {
  let def: any = {}
  try {
    def = JSON.parse(definitionJson)
  } catch (e) {
    console.error('Invalid JSON definition', e)
    return { nodes: [], edges: [] }
  }

  const nodes: Node[] = []
  const edges: Edge[] = []
  const rawNodes = def.nodes || {}

  // Create Nodes
  Object.keys(rawNodes).forEach((key) => {
    const nodeData = rawNodes[key]
    nodes.push({
      id: key,
      data: { label: key, ...nodeData },
      position: { x: 0, y: 0 }, // Will be set by dagre
      type: 'custom', // Use our custom node
    })
  })

  const commonEdgeStyle = {
    type: 'smoothstep',
    markerEnd: { type: MarkerType.ArrowClosed, color: '#333' },
    animated: false,
    style: { stroke: '#333', strokeWidth: 2 },
  }

  // Create Edges
  if (def.edges) {
    // Use explicit edges definition
    def.edges.forEach((edge: any) => {
      edges.push({
        id: `${edge.from}-${edge.to}-${edge.action || 'default'}`,
        source: edge.from,
        target: edge.to,
        label: edge.action,
        ...commonEdgeStyle,
      })
    })
  } else {
    // Fallback: Infer Edges from nodes
    Object.keys(rawNodes).forEach((key) => {
      const nodeData = rawNodes[key]

      // Handle 'post' actions
      if (nodeData.post) {
        if (nodeData.post.action_static) {
          edges.push({
            id: `${key}-${nodeData.post.action_static}`,
            source: key,
            target: nodeData.post.action_static,
            ...commonEdgeStyle,
          })
        }
        if (nodeData.post.action_map) {
          Object.entries(nodeData.post.action_map).forEach(
            ([val, target]: [string, any]) => {
              edges.push({
                id: `${key}-${target}-${val}`,
                source: key,
                target: target as string,
                label: val,
                ...commonEdgeStyle,
              })
            }
          )
        }
      }

      // Handle 'choice' cases
      if (nodeData.kind === 'choice' && nodeData.choice_cases) {
        nodeData.choice_cases.forEach((c: any, idx: number) => {
          if (c.action) {
            let label = `Case ${idx + 1}`
            if (c.cond_eq) label = `== ${c.cond_eq}`
            else if (c.cond_gt) label = `> ${c.cond_gt}`
            // Add more condition labels as needed

            edges.push({
              id: `${key}-${c.action}-${idx}`,
              source: key,
              target: c.action,
              label: label,
              ...commonEdgeStyle,
            })
          }
        })
      }
    })
  }

  // Handle Start Node Indicator
  if (def.start) {
    // We can add a special start node or style the start node differently
    const startNode = nodes.find((n) => n.id === def.start)
    if (startNode) {
      // Mark as start node in data, so CustomNode can render it differently if needed
      startNode.data.isStart = true
      startNode.style = {
        ...startNode.style,
        background: '#e6f7ff',
        borderColor: '#1890ff',
        borderWidth: '2px',
      }
      // Removed label modification
    }
  }

  return getLayoutedElements(nodes, edges)
}
