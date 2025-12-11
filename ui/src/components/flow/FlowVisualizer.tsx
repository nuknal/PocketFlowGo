import { useEffect } from 'react'
import {
  ReactFlow,
  Controls,
  Background,
  useNodesState,
  useEdgesState,
  Panel,
  type Node,
  type Edge,
  type ColorMode,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { expandFlowDefinition } from './utils'
import CustomNode from './CustomNode'
import GroupNode from './GroupNode'
import { useTheme } from '../theme-provider'

interface FlowVisualizerProps {
  definitionJson: string
  height?: string
}

const nodeTypes = {
  custom: CustomNode,
  group: GroupNode,
}

export default function FlowVisualizer({
  definitionJson,
  height = '500px',
}: FlowVisualizerProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([])
  const { theme } = useTheme()

  useEffect(() => {
    const loadFlow = async () => {
      if (!definitionJson) {
        setNodes([])
        setEdges([])
        return
      }
      const { nodes: newNodes, edges: newEdges } = await expandFlowDefinition(
        definitionJson
      )
      setNodes(newNodes)
      setEdges(newEdges)
    }
    loadFlow()
  }, [definitionJson, setNodes, setEdges])

  return (
    <div className="w-full border rounded-lg bg-background" style={{ height }}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        nodeTypes={nodeTypes}
        colorMode={theme as ColorMode}
        fitView
      >
        <Background />
        <Controls />
        <Panel
          position="top-right"
          className="bg-card text-card-foreground p-2 rounded shadow text-xs border"
        >
          Nodes: {nodes.length} | Edges: {edges.length}
        </Panel>
      </ReactFlow>
    </div>
  )
}
