import { Link } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Search, Plus } from 'lucide-react'
import { api, type Task, type Flow } from '@/lib/api'

const getStatusBadge = (status: string) => {
  switch (status) {
    case 'completed':
      return <Badge className="bg-green-500">Completed</Badge>
    case 'running':
      return <Badge className="bg-blue-500">Running</Badge>
    case 'failed':
      return <Badge variant="destructive">Failed</Badge>
    case 'pending':
      return <Badge variant="secondary">Pending</Badge>
    case 'canceling':
      return <Badge className="bg-orange-500">Canceling</Badge>
    case 'canceled':
      return <Badge variant="secondary">Canceled</Badge>
    default:
      return <Badge variant="outline">{status}</Badge>
  }
}

export default function Tasks() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [flows, setFlows] = useState<Flow[]>([])
  const [filteredTasks, setFilteredTasks] = useState<Task[]>([])
  const [statusFilter, setStatusFilter] = useState('all')
  const [searchQuery, setSearchQuery] = useState('')

  // Create Task State
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [selectedFlowId, setSelectedFlowId] = useState('')
  const [taskParams, setTaskParams] = useState('{}')

  const fetchData = async () => {
    try {
      const [tasksData, flowsData] = await Promise.all([
        api.getTasks(),
        api.getFlows().catch(() => []),
      ])
      setTasks(tasksData || [])
      setFlows(flowsData || [])
    } catch (error) {
      console.error('Failed to fetch data:', error)
    }
  }

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 5000)
    return () => clearInterval(interval)
  }, [])

  useEffect(() => {
    let result = tasks

    if (statusFilter !== 'all') {
      result = result.filter((t) => t.status === statusFilter)
    }

    if (searchQuery) {
      result = result.filter((t) =>
        t.id.toLowerCase().includes(searchQuery.toLowerCase())
      )
    }

    setFilteredTasks(result)
  }, [tasks, statusFilter, searchQuery])

  const handleCreateTask = async () => {
    if (!selectedFlowId) return
    setCreating(true)
    try {
      let params = {}
      try {
        params = JSON.parse(taskParams)
      } catch (e) {
        alert('Invalid JSON parameters')
        setCreating(false)
        return
      }

      await api.createTask(selectedFlowId, 0, params)
      setIsDialogOpen(false)
      setSelectedFlowId('')
      setTaskParams('{}')
      fetchData()
    } catch (error) {
      console.error('Failed to create task:', error)
      alert('Failed to create task')
    } finally {
      setCreating(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Tasks</h1>
          <p className="text-muted-foreground">
            Monitor and manage task execution
          </p>
        </div>
        <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="mr-2 h-4 w-4" />
              Create Task
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-[500px]">
            <DialogHeader>
              <DialogTitle>Create New Task</DialogTitle>
              <DialogDescription>
                Start a new execution instance of a flow.
              </DialogDescription>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="grid grid-cols-4 items-center gap-4">
                <Label htmlFor="flow" className="text-right">
                  Flow
                </Label>
                <div className="col-span-3">
                  <Select
                    value={selectedFlowId}
                    onValueChange={setSelectedFlowId}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="Select a flow" />
                    </SelectTrigger>
                    <SelectContent>
                      {flows.map((flow) => (
                        <SelectItem key={flow.id} value={flow.id}>
                          {flow.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <div className="grid grid-cols-4 items-start gap-4">
                <Label htmlFor="params" className="text-right pt-2">
                  Params (JSON)
                </Label>
                <Textarea
                  id="params"
                  value={taskParams}
                  onChange={(e) => setTaskParams(e.target.value)}
                  className="col-span-3 font-mono text-xs h-[150px]"
                />
              </div>
            </div>
            <DialogFooter>
              <Button
                type="submit"
                onClick={handleCreateTask}
                disabled={creating || !selectedFlowId}
              >
                {creating ? 'Creating...' : 'Create Task'}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Task History</CardTitle>
              <CardDescription>
                View and filter task execution history
              </CardDescription>
            </div>
            <div className="flex items-center gap-2">
              <div className="relative">
                <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search tasks..."
                  className="pl-8 w-[250px]"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>
              <Select value={statusFilter} onValueChange={setStatusFilter}>
                <SelectTrigger className="w-[180px]">
                  <SelectValue placeholder="Status" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Status</SelectItem>
                  <SelectItem value="running">Running</SelectItem>
                  <SelectItem value="completed">Completed</SelectItem>
                  <SelectItem value="failed">Failed</SelectItem>
                  <SelectItem value="pending">Pending</SelectItem>
                  <SelectItem value="canceling">Canceling</SelectItem>
                  <SelectItem value="canceled">Canceled</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Task ID</TableHead>
                <TableHead>Flow Version</TableHead>
                <TableHead>Current Node</TableHead>
                <TableHead>Step</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Updated At</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredTasks.map((task) => (
                <TableRow key={task.id}>
                  <TableCell className="font-medium font-mono text-xs">
                    <Link to={`/tasks/${task.id}`} className="hover:underline">
                      {task.id}
                    </Link>
                  </TableCell>
                  <TableCell>
                    {task.flow_name ? (
                      <div className="flex flex-col">
                        <Link
                          to={`/flow-versions/${task.flow_version_id}`}
                          className="font-medium hover:underline hover:text-blue-600"
                        >
                          {task.flow_name}-v{task.flow_version}
                        </Link>
                        <span className="text-[10px] text-muted-foreground truncate max-w-[100px]">
                          {task.flow_version_id}
                        </span>
                      </div>
                    ) : (
                      <Link
                        to={`/flow-versions/${task.flow_version_id}`}
                        className="hover:underline hover:text-blue-600"
                      >
                        {task.flow_version_id}
                      </Link>
                    )}
                  </TableCell>
                  <TableCell>{task.current_node_key || '-'}</TableCell>
                  <TableCell>{task.step_count}</TableCell>
                  <TableCell>{getStatusBadge(task.status)}</TableCell>
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
              {filteredTasks.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={7}
                    className="text-center h-24 text-muted-foreground"
                  >
                    No tasks found matching your filters.
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
