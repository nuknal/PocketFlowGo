import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { ArrowLeft } from 'lucide-react'
import { api, type Task, type NodeRun } from '@/lib/api'

export default function TaskDetails() {
  const { id } = useParams<{ id: string }>()
  const [task, setTask] = useState<Task | null>(null)
  const [runs, setRuns] = useState<NodeRun[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!id) return

    const fetchData = async () => {
      try {
        const [taskData, runsData] = await Promise.all([
          api.getTask(id),
          api.getTaskRuns(id),
        ])
        setTask(taskData)
        setRuns(runsData || [])
      } catch (error) {
        console.error('Failed to fetch task details:', error)
      } finally {
        setLoading(false)
      }
    }

    fetchData()
    const interval = setInterval(async () => {
      try {
        const [taskData, runsData] = await Promise.all([
          api.getTask(id),
          api.getTaskRuns(id),
        ])
        setTask(taskData)
        setRuns(runsData || [])
      } catch (error) {
        console.error('Failed to refresh task details:', error)
      }
    }, 3000) // Auto-refresh every 3s

    return () => clearInterval(interval)
  }, [id])

  if (loading) {
    return <div className="p-8 text-center">Loading task details...</div>
  }

  if (!task) {
    return <div className="p-8 text-center">Task not found</div>
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/tasks">
          <Button variant="outline" size="icon">
            <ArrowLeft className="h-4 w-4" />
          </Button>
        </Link>
        <div>
          <h1 className="text-3xl font-bold">Task Details</h1>
          <p className="text-muted-foreground font-mono">{task.id}</p>
        </div>
        <div className="ml-auto flex gap-2">
          <Badge
            variant={
              task.status === 'completed'
                ? 'default'
                : task.status === 'failed'
                ? 'destructive'
                : 'secondary'
            }
          >
            {task.status.toUpperCase()}
          </Badge>
        </div>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Overview</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            <div className="flex justify-between py-1 border-b">
              <span className="text-muted-foreground">Flow Version</span>
              <span className="font-medium text-right">
                {task.flow_name ? (
                  <>
                    <Link
                      to={`/flow-versions/${task.flow_version_id}`}
                      className="hover:underline hover:text-blue-600"
                    >
                      {task.flow_name}-v{task.flow_version}
                    </Link>
                    <div className="text-[10px] text-muted-foreground">
                      {task.flow_version_id}
                    </div>
                  </>
                ) : (
                  <Link
                    to={`/flow-versions/${task.flow_version_id}`}
                    className="hover:underline hover:text-blue-600"
                  >
                    {task.flow_version_id}
                  </Link>
                )}
              </span>
            </div>
            <div className="flex justify-between py-1 border-b">
              <span className="text-muted-foreground">Current Node</span>
              <span className="font-medium">
                {task.current_node_key || '-'}
              </span>
            </div>
            <div className="flex justify-between py-1 border-b">
              <span className="text-muted-foreground">Step Count</span>
              <span className="font-medium">{task.step_count}</span>
            </div>
            <div className="flex justify-between py-1 border-b">
              <span className="text-muted-foreground">Created At</span>
              <span className="font-medium">
                {new Date(task.created_at * 1000).toLocaleString()}
              </span>
            </div>
            <div className="flex justify-between py-1 border-b">
              <span className="text-muted-foreground">Updated At</span>
              <span className="font-medium">
                {new Date(task.updated_at * 1000).toLocaleString()}
              </span>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Data</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <h4 className="text-sm font-semibold mb-1">Parameters</h4>
              <pre className="bg-muted p-2 rounded text-xs overflow-auto max-h-[150px]">
                {JSON.stringify(JSON.parse(task.params_json || '{}'), null, 2)}
              </pre>
            </div>
            <div>
              <h4 className="text-sm font-semibold mb-1">Shared State</h4>
              <pre className="bg-muted p-2 rounded text-xs overflow-auto max-h-[150px]">
                {JSON.stringify(JSON.parse(task.shared_json || '{}'), null, 2)}
              </pre>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Execution History (Node Runs)</CardTitle>
          <CardDescription>
            Trace of executed nodes for this task.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Node Key</TableHead>
                <TableHead>Attempt</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Worker</TableHead>
                <TableHead>Started At</TableHead>
                <TableHead>Duration</TableHead>
                <TableHead className="text-right">Action</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {runs.map((run) => (
                <TableRow key={run.id}>
                  <TableCell className="font-medium">{run.node_key}</TableCell>
                  <TableCell>{run.attempt_no}</TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        run.status === 'ok'
                          ? 'default'
                          : run.status === 'error'
                          ? 'destructive'
                          : 'secondary'
                      }
                    >
                      {run.status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {run.worker_id}
                  </TableCell>
                  <TableCell>
                    {new Date(run.started_at * 1000).toLocaleTimeString()}
                  </TableCell>
                  <TableCell>
                    {run.finished_at > 0
                      ? `${(run.finished_at - run.started_at).toFixed(3)}s`
                      : '-'}
                  </TableCell>
                  <TableCell className="text-right">
                    {run.action || '-'}
                  </TableCell>
                </TableRow>
              ))}
              {runs.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={7}
                    className="text-center h-24 text-muted-foreground"
                  >
                    No execution history found.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
