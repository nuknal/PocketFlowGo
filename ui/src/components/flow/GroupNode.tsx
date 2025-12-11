import { type NodeProps } from '@xyflow/react'

export default function GroupNode({ data }: NodeProps) {
  return (
    <div className="w-full h-full border-2 border-dashed border-blue-200 dark:border-blue-800 bg-blue-50/20 dark:bg-blue-900/20 rounded-lg p-4 relative">
      <div className="absolute -top-3 left-4 bg-background px-2 text-xs font-mono text-blue-500 dark:text-blue-400 border border-blue-200 dark:border-blue-800 rounded">
        {data.label as string}
      </div>
    </div>
  )
}
