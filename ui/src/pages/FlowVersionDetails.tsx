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
import { api, type FlowVersion, type Task } from '@/lib/api'
import FlowVisualizer from '@/components/flow/FlowVisualizer'

export default function FlowVersionDetails() {
  const { id } = useParams<{ id: string }>()
  const [version, setVersion] = useState<FlowVersion | null>(null)
  const [tasks, setTasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!id) return

    const fetchData = async () => {
      try {
        const [versionData, tasksData] = await Promise.all([
          api.getFlowVersion(id),
          api.getTasks(undefined, id),
        ])
        setVersion(versionData)
        setTasks(tasksData || [])
      } catch (error) {
        console.error('Failed to fetch details:', error)
      } finally {
        setLoading(false)
      }
    }

    fetchData()
    const interval = setInterval(async () => {
      try {
        const tasksData = await api.getTasks(undefined, id)
        setTasks(tasksData || [])
      } catch (e) {
        console.error(e)
      }
    }, 5000)
    return () => clearInterval(interval)
  }, [id])

  if (loading) return <div className="p-8 text-center">Loading...</div>
  if (!version) return <div className="p-8 text-center">Version not found</div>

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to={`/flows/${version.flow_id}`}>
          <Button variant="outline" size="icon">
            <ArrowLeft className="h-4 w-4" />
          </Button>
        </Link>
        <div>
          <h1 className="text-3xl font-bold">Flow Version Details</h1>
          <p className="text-muted-foreground">
            Version: {version.version} | ID: {version.id}
          </p>
        </div>
        <div className="ml-auto">
          <Badge
            variant={version.status === 'published' ? 'default' : 'secondary'}
          >
            {version.status.toUpperCase()}
          </Badge>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Visualization</CardTitle>
          <CardDescription>
            Visual representation of the workflow logic.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <FlowVisualizer definitionJson={version.definition_json} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Definition</CardTitle>
          <CardDescription>
            The workflow definition JSON for this version.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <pre className="bg-muted p-4 rounded text-xs font-mono overflow-auto max-h-[300px]">
            {JSON.stringify(
              JSON.parse(version.definition_json || '{}'),
              null,
              2
            )}
          </pre>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Executions (Tasks)</CardTitle>
          <CardDescription>
            List of tasks executed using this version.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Task ID</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Current Node</TableHead>
                <TableHead>Steps</TableHead>
                <TableHead>Updated At</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {tasks.map((task) => (
                <TableRow key={task.id}>
                  <TableCell className="font-medium font-mono text-xs">
                    <Link to={`/tasks/${task.id}`} className="hover:underline">
                      {task.id}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        task.status === 'completed'
                          ? 'default'
                          : task.status === 'failed'
                          ? 'destructive'
                          : task.status === 'running'
                          ? 'secondary'
                          : 'outline'
                      }
                    >
                      {task.status}
                    </Badge>
                  </TableCell>
                  <TableCell>{task.current_node_key || '-'}</TableCell>
                  <TableCell>{task.step_count}</TableCell>
                  <TableCell>
                    {new Date(task.updated_at * 1000).toLocaleString()}
                  </TableCell>
                  <TableCell className="text-right">
                    <Link to={`/tasks/${task.id}`}>
                      <Button variant="ghost" size="sm">
                        Details
                      </Button>
                    </Link>
                  </TableCell>
                </TableRow>
              ))}
              {tasks.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={6}
                    className="text-center h-24 text-muted-foreground"
                  >
                    No executions found for this version.
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
