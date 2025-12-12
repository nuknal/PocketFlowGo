import { useEffect, useState } from 'react'
import { Badge } from '@/components/ui/badge'
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
import { api, type Worker } from '@/lib/api'

export default function Workers() {
  const [workers, setWorkers] = useState<Worker[]>([])

  useEffect(() => {
    const fetchData = async () => {
      try {
        const workersData = await api.getWorkers()
        setWorkers(workersData || [])
      } catch (error) {
        console.error('Failed to fetch workers:', error)
      }
    }

    fetchData()
    const interval = setInterval(fetchData, 5000)
    return () => clearInterval(interval)
  }, [])

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Workers</h1>
          <p className="text-muted-foreground">
            Monitor registered worker nodes
          </p>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Worker Nodes</CardTitle>
          <CardDescription>Live status of connected workers.</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Worker ID</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>URL</TableHead>
                <TableHead>Services</TableHead>
                <TableHead>Current Load</TableHead>
                <TableHead>Last Heartbeat</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {workers.map((worker) => (
                <TableRow key={worker.id}>
                  <TableCell className="font-medium">{worker.id}</TableCell>
                  <TableCell>
                    <Badge variant="outline">{worker.type || 'http'}</Badge>
                  </TableCell>
                  <TableCell className="font-mono text-xs">
                    {worker.url}
                  </TableCell>
                  <TableCell>
                    <div className="flex gap-1 flex-wrap">
                      {worker.services?.map((s) => (
                        <Badge key={s} variant="outline" className="text-xs">
                          {s}
                        </Badge>
                      ))}
                    </div>
                  </TableCell>
                  <TableCell>{worker.load}</TableCell>
                  <TableCell>
                    {new Date(worker.last_heartbeat * 1000).toLocaleString()}
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        worker.status === 'online' ? 'default' : 'secondary'
                      }
                    >
                      {worker.status}
                    </Badge>
                  </TableCell>
                </TableRow>
              ))}
              {workers.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={7}
                    className="text-center h-24 text-muted-foreground"
                  >
                    No active workers found.
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
