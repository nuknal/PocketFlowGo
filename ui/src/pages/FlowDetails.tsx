import { useEffect, useState } from 'react'
import { useParams, Link, useSearchParams } from 'react-router-dom'
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { ArrowLeft, Plus, Play } from 'lucide-react'
import { api, type FlowVersion, type Task } from '@/lib/api'
import { RunTaskDialog } from '@/components/RunTaskDialog'
import FlowVisualizer from '@/components/flow/FlowVisualizer'

export default function FlowDetails() {
  const { id } = useParams<{ id: string }>()
  const [searchParams] = useSearchParams()
  const versionParam = searchParams.get('version')
  const [versions, setVersions] = useState<FlowVersion[]>([])
  const [selectedVersionId, setSelectedVersionId] = useState<string>('')
  const [loading, setLoading] = useState(true)
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [newVersion, setNewVersion] = useState('')
  const [newDefinition, setNewDefinition] = useState('')
  const [creating, setCreating] = useState(false)
  const [isRunDialogOpen, setIsRunDialogOpen] = useState(false)
  const [tasks, setTasks] = useState<Task[]>([])

  const fetchVersions = async () => {
    if (!id) return
    try {
      const data = await api.getFlowVersions(id)
      const sortedVersions = data || []
      setVersions(sortedVersions)
      if (sortedVersions.length > 0 && !selectedVersionId) {
        if (versionParam) {
          const targetVersion = sortedVersions.find(
            (v) => v.id === versionParam
          )
          if (targetVersion) {
            setSelectedVersionId(targetVersion.id)
          } else {
            setSelectedVersionId(sortedVersions[0].id)
          }
        } else {
          setSelectedVersionId(sortedVersions[0].id)
        }
      }
    } catch (error) {
      console.error('Failed to fetch flow versions:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchVersions()
  }, [id])

  const selectedVersion = versions.find((v) => v.id === selectedVersionId)

  useEffect(() => {
    if (selectedVersion) {
      // Fetch tasks for this version
      const fetchTasks = async () => {
        try {
          const data = await api.getTasks(undefined, selectedVersion.id, 1, 5)
          setTasks(data.data || [])
        } catch (error) {
          console.error('Failed to fetch tasks:', error)
        }
      }
      fetchTasks()
      const interval = setInterval(fetchTasks, 5000)
      return () => clearInterval(interval)
    }
  }, [selectedVersion])

  const handleCreateVersion = async () => {
    if (!id || !newVersion || !newDefinition) return
    setCreating(true)
    try {
      await api.createFlowVersion(id, parseInt(newVersion), newDefinition)
      setNewVersion('')
      setNewDefinition('')
      setIsDialogOpen(false)
      fetchVersions()
    } catch (error) {
      console.error('Failed to create flow version:', error)
    } finally {
      setCreating(false)
    }
  }

  const defaultDefinition = JSON.stringify(
    {
      start: 'start_node',
      nodes: {
        start_node: {
          kind: 'executor',
          service: 'transform',
          params: { op: 'upper' },
          prep: { input_key: 'input' },
          post: { output_key: 'output', action_static: 'next' },
        },
      },
      edges: [],
    },
    null,
    2
  )

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/flows">
          <Button variant="outline" size="icon">
            <ArrowLeft className="h-4 w-4" />
          </Button>
        </Link>
        <div>
          <h1 className="text-3xl font-bold">Flow Details</h1>
          <p className="text-muted-foreground">ID: {id}</p>
        </div>
        <div className="ml-auto flex items-center gap-2">
          {versions.length > 0 && (
            <Select
              value={selectedVersionId}
              onValueChange={setSelectedVersionId}
            >
              <SelectTrigger className="w-[180px]">
                <SelectValue placeholder="Select Version" />
              </SelectTrigger>
              <SelectContent>
                {versions.map((v) => (
                  <SelectItem key={v.id} value={v.id}>
                    v{v.version} ({v.status})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}

          {selectedVersion && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => setIsRunDialogOpen(true)}
            >
              <Play className="h-4 w-4 mr-2" />
              Run
            </Button>
          )}

          <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
            <DialogTrigger asChild>
              <Button onClick={() => setNewDefinition(defaultDefinition)}>
                <Plus className="mr-2 h-4 w-4" />
                Create Version
              </Button>
            </DialogTrigger>
            <DialogContent className="sm:max-w-[600px]">
              <DialogHeader>
                <DialogTitle>Create Flow Version</DialogTitle>
                <DialogDescription>
                  Define a new version for this flow.
                </DialogDescription>
              </DialogHeader>
              <div className="grid gap-4 py-4">
                <div className="grid grid-cols-4 items-center gap-4">
                  <Label htmlFor="version" className="text-right">
                    Version
                  </Label>
                  <Input
                    id="version"
                    type="number"
                    value={newVersion}
                    onChange={(e) => setNewVersion(e.target.value)}
                    placeholder="e.g. 1"
                    className="col-span-3"
                  />
                </div>
                <div className="grid grid-cols-4 items-start gap-4">
                  <Label htmlFor="definition" className="text-right pt-2">
                    Definition (JSON)
                  </Label>
                  <Textarea
                    id="definition"
                    value={newDefinition}
                    onChange={(e) => setNewDefinition(e.target.value)}
                    className="col-span-3 h-[300px] font-mono text-xs"
                  />
                </div>
              </div>
              <DialogFooter>
                <Button
                  type="submit"
                  onClick={handleCreateVersion}
                  disabled={creating}
                >
                  {creating ? 'Creating...' : 'Create'}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      </div>

      {loading ? (
        <div className="text-center py-8">Loading...</div>
      ) : !selectedVersion ? (
        <div className="text-center py-8 text-muted-foreground">
          No versions found. Create a new version to get started.
        </div>
      ) : (
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle>
                    Visualization (v{selectedVersion.version})
                  </CardTitle>
                  <CardDescription>
                    Status: {selectedVersion.status.toUpperCase()}
                  </CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <FlowVisualizer
                definitionJson={selectedVersion.definition_json}
                height="150px"
              />
            </CardContent>
          </Card>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <Card className="md:col-span-1">
              <CardHeader>
                <CardTitle>Definition</CardTitle>
              </CardHeader>
              <CardContent>
                <pre className="bg-muted p-4 rounded text-xs font-mono overflow-auto max-h-[400px]">
                  {JSON.stringify(
                    JSON.parse(selectedVersion.definition_json || '{}'),
                    null,
                    2
                  )}
                </pre>
              </CardContent>
            </Card>

            <Card className="md:col-span-1">
              <CardHeader>
                <CardTitle>Recent Executions</CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Task ID</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead className="text-right">Action</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {tasks.slice(0, 5).map((task) => (
                      <TableRow key={task.id}>
                        <TableCell className="font-mono text-xs">
                          <Link
                            to={`/tasks/${task.id}`}
                            className="hover:underline"
                          >
                            {task.id.substring(0, 8)}...
                          </Link>
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant={
                              task.status === 'completed'
                                ? 'default'
                                : task.status === 'failed'
                                ? 'destructive'
                                : 'secondary'
                            }
                            className="text-[10px] px-1 h-5"
                          >
                            {task.status}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-right">
                          <Link to={`/tasks/${task.id}`}>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-6 w-6 p-0"
                            >
                              <ArrowLeft className="h-3 w-3 rotate-180" />
                            </Button>
                          </Link>
                        </TableCell>
                      </TableRow>
                    ))}
                    {tasks.length === 0 && (
                      <TableRow>
                        <TableCell
                          colSpan={3}
                          className="text-center text-muted-foreground text-xs py-4"
                        >
                          No executions yet.
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </div>
        </div>
      )}

      {selectedVersion && id && (
        <RunTaskDialog
          open={isRunDialogOpen}
          onOpenChange={setIsRunDialogOpen}
          flowId={id}
          version={selectedVersion.version}
          flowName={`Flow ${id}`}
          onSuccess={() => setIsRunDialogOpen(false)}
        />
      )}
    </div>
  )
}
