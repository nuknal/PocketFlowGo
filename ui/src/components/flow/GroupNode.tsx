import { type NodeProps } from '@xyflow/react'

export default function GroupNode({ data }: NodeProps) {
  return (
    <div className="w-full h-full border-2 border-dashed border-blue-200 bg-blue-50/20 rounded-lg p-4 relative">
      <div className="absolute -top-3 left-4 bg-white px-2 text-xs font-mono text-blue-400 border border-blue-100 rounded">
        {data.label as string}
      </div>
    </div>
  )
}
