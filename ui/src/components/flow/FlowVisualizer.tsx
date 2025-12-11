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
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { expandFlowDefinition } from './utils'
import CustomNode from './CustomNode'
import GroupNode from './GroupNode'

interface FlowVisualizerProps {
  definitionJson: string
}

const nodeTypes = {
  custom: CustomNode,
  group: GroupNode,
}

export default function FlowVisualizer({
  definitionJson,
}: FlowVisualizerProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([])

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
    <div className="h-[500px] w-full border rounded-lg bg-slate-50">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        nodeTypes={nodeTypes}
        fitView
      >
        <Background />
        <Controls />
        <Panel
          position="top-right"
          className="bg-white p-2 rounded shadow text-xs"
        >
          Nodes: {nodes.length} | Edges: {edges.length}
        </Panel>
      </ReactFlow>
    </div>
  )
}
