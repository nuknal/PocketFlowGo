import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

const NodeHeader = ({
  kind,
  label,
  isStart,
}: {
  kind: string
  label: string
  isStart?: boolean
}) => {
  const getBadgeVariant = (k: string) => {
    switch (k) {
      case 'executor':
        return 'default'
      case 'choice':
        return 'secondary'
      case 'parallel':
        return 'outline' // Blue-ish in default theme?
      case 'foreach':
        return 'outline'
      case 'subflow':
        return 'destructive' // Or something distinct
      case 'approval':
        return 'secondary'
      default:
        return 'outline'
    }
  }

  return (
    <CardHeader className="p-2 pb-1">
      <CardTitle className="text-sm font-medium flex items-center justify-between gap-2">
        <span
          className="truncate max-w-[120px] flex items-center gap-1"
          title={label}
        >
          {isStart && (
            <Badge className="text-[8px] h-4 px-1 bg-blue-500 hover:bg-blue-600">
              Start
            </Badge>
          )}
          {label}
        </span>
        <Badge variant={getBadgeVariant(kind)} className="text-[10px] px-1 h-5">
          {kind}
        </Badge>
      </CardTitle>
    </CardHeader>
  )
}

export default function CustomNode({ data }: NodeProps) {
  const { label, kind, isStart, ...rest } = data as any

  return (
    <Card
      className={`min-w-[180px] max-w-[250px] shadow-sm border-2 ${
        isStart
          ? 'border-blue-500 dark:border-blue-400 ring-2 ring-blue-100 dark:ring-blue-900'
          : 'border-border'
      }`}
    >
      <Handle
        type="target"
        position={Position.Left}
        className="w-3 h-3 bg-muted-foreground"
      />

      <NodeHeader kind={kind} label={label as string} isStart={isStart} />

      <CardContent className="p-2 pt-1 text-xs space-y-1">
        {/* Service / Exec Type */}
        {(rest.service || rest.exec_type) && (
          <div className="flex flex-col gap-0.5">
            <span className="text-muted-foreground font-mono text-[10px]">
              Service
            </span>
            <div className="font-medium bg-muted p-1 rounded">
              {rest.service || `${rest.exec_type}:${rest.func || ''}`}
            </div>
          </div>
        )}

        {/* Parallel Details */}
        {kind === 'parallel' && (
          <>
            <div className="flex gap-1">
              <Badge variant="outline" className="text-[9px] h-4">
                {rest.parallel_mode}
              </Badge>
              {rest.max_parallel && (
                <Badge variant="outline" className="text-[9px] h-4">
                  Max: {rest.max_parallel}
                </Badge>
              )}
            </div>
            {rest.parallel_services && (
              <div className="flex flex-wrap gap-1 mt-1">
                {rest.parallel_services.map((s: string, i: number) => (
                  <span
                    key={i}
                    className="bg-blue-50 text-blue-700 dark:bg-blue-900/50 dark:text-blue-200 px-1 rounded text-[10px]"
                  >
                    {s}
                  </span>
                ))}
              </div>
            )}
            {rest.parallel_execs && (
              <div className="space-y-1 mt-1">
                {rest.parallel_execs.map((e: any, i: number) => (
                  <div
                    key={i}
                    className="bg-slate-50 dark:bg-slate-800 p-1 rounded border border-slate-100 dark:border-slate-700 text-[10px]"
                  >
                    <div className="font-medium">
                      {e.service ||
                        `${e.exec_type}${e.func ? `:${e.func}` : ''}`}
                    </div>
                    {e.params && (
                      <div className="text-muted-foreground text-[9px] truncate max-w-[180px]">
                        {JSON.stringify(e.params)}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </>
        )}

        {/* Foreach Details */}
        {kind === 'foreach' && (
          <>
            <div className="flex gap-1">
              <Badge variant="outline" className="text-[9px] h-4">
                {rest.parallel_mode}
              </Badge>
            </div>
            {rest.foreach_execs && rest.foreach_execs.length > 0 && (
              <div className="mt-1 space-y-1">
                <span className="text-muted-foreground text-[10px]">
                  Overrides:
                </span>
                {rest.foreach_execs.map((e: any, i: number) => (
                  <div
                    key={i}
                    className="bg-slate-50 dark:bg-slate-800 p-1 rounded border border-slate-100 dark:border-slate-700 text-[10px]"
                  >
                    <div className="font-mono text-[9px]">Index {e.index}</div>
                    {e.params && (
                      <div className="text-muted-foreground text-[9px] truncate max-w-[180px]">
                        {JSON.stringify(e.params)}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </>
        )}

        {/* Subflow Details */}
        {kind === 'subflow' && rest.flow_id && (
          <div className="mt-1">
            <span className="text-muted-foreground text-[10px]">Flow ID:</span>
            <div
              className="font-mono bg-slate-100 dark:bg-slate-800 p-1 rounded truncate"
              title={rest.flow_id}
            >
              {rest.flow_id}
            </div>
          </div>
        )}

        {/* Wait Event Details */}
        {kind === 'wait_event' && rest.params?.signal_key && (
          <div className="mt-1">
            <span className="text-muted-foreground text-[10px]">Wait For:</span>
            <div
              className="font-mono bg-yellow-50 dark:bg-yellow-900/30 p-1 rounded truncate text-yellow-800 dark:text-yellow-200"
              title={rest.params.signal_key}
            >
              {rest.params.signal_key}
            </div>
          </div>
        )}
      </CardContent>

      <Handle
        type="source"
        position={Position.Right}
        className="w-3 h-3 bg-slate-400 dark:bg-slate-500"
      />
    </Card>
  )
}
