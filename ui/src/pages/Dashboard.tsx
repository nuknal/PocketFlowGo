import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Activity, CheckCircle2, Clock, XCircle } from 'lucide-react'
import { api, type Task, type Worker } from '@/lib/api'

export default function Dashboard() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [workers, setWorkers] = useState<Worker[]>([])
  const [loading, setLoading] = useState(true)
  const [counts, setCounts] = useState({
    total: 0,
    pending: 0,
    failed: 0,
  })

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [recentTasks, pending, failed, workersData] = await Promise.all([
          api.getTasks(undefined, undefined, 1, 5),
          api.getTasks('pending', undefined, 1, 1),
          api.getTasks('failed', undefined, 1, 1),
          api.getWorkers(),
        ])
        setTasks(recentTasks.data || [])
        setCounts({
          total: recentTasks.total,
          pending: pending.total,
          failed: failed.total,
        })
        setWorkers(workersData || [])
      } catch (error) {
        console.error('Failed to fetch dashboard data:', error)
      } finally {
        setLoading(false)
      }
    }

    fetchData()
    const interval = setInterval(fetchData, 5000)
    return () => clearInterval(interval)
  }, [])

  const stats = {
    totalTasks: counts.total,
    activeWorkers: workers.filter((w) => w.status === 'online').length,
    pendingTasks: counts.pending,
    failedTasks: counts.failed,
  }

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Dashboard</h1>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Tasks</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {loading ? '-' : stats.totalTasks}
            </div>
            <p className="text-xs text-muted-foreground">All time tasks</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Active Workers
            </CardTitle>
            <CheckCircle2 className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {loading ? '-' : stats.activeWorkers}
            </div>
            <p className="text-xs text-muted-foreground">Online nodes</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Pending Tasks</CardTitle>
            <Clock className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {loading ? '-' : stats.pendingTasks}
            </div>
            <p className="text-xs text-muted-foreground">
              Waiting for execution
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Failed Tasks</CardTitle>
            <XCircle className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {loading ? '-' : stats.failedTasks}
            </div>
            <p className="text-xs text-muted-foreground">Requires attention</p>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-7">
        <Card className="col-span-4">
          <CardHeader>
            <CardTitle>Recent Activity</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {tasks.slice(0, 5).map((task) => (
                <Link
                  key={task.id}
                  to={`/tasks/${task.id}`}
                  className="flex items-center p-2 rounded-md hover:bg-muted/50 transition-colors"
                >
                  <div className="ml-4 space-y-1">
                    <p className="text-sm font-medium leading-none">
                      Task {task.id}
                    </p>
                    <p className="text-sm text-muted-foreground">
                      Status: {task.status} | Step: {task.step_count}
                    </p>
                  </div>
                  <div className="ml-auto font-medium text-xs text-muted-foreground">
                    {new Date(task.updated_at * 1000).toLocaleString()}
                  </div>
                </Link>
              ))}
              {tasks.length === 0 && (
                <p className="text-sm text-muted-foreground">
                  No recent activity
                </p>
              )}
            </div>
          </CardContent>
        </Card>
        <Card className="col-span-3">
          <CardHeader>
            <CardTitle>Worker Health</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {workers.slice(0, 5).map((worker) => (
                <div
                  key={worker.id}
                  className="flex items-center justify-between"
                >
                  <span className="text-sm font-medium">{worker.id}</span>
                  <span
                    className={`text-xs px-2 py-1 rounded-full ${
                      worker.status === 'online'
                        ? 'bg-green-100 text-green-800'
                        : 'bg-gray-100 text-gray-800'
                    }`}
                  >
                    {worker.status} (Load: {worker.load})
                  </span>
                </div>
              ))}
              {workers.length === 0 && (
                <p className="text-sm text-muted-foreground">
                  No workers connected
                </p>
              )}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
