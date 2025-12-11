import { useEffect, useState } from 'react'
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
import { Plus } from 'lucide-react'
import { Link } from 'react-router-dom'
import { api, type Flow } from '@/lib/api'

export default function Flows() {
  const [flows, setFlows] = useState<Flow[]>([])

  useEffect(() => {
    const fetchFlows = async () => {
      try {
        const data = await api.getFlows()
        setFlows(data || [])
      } catch (error) {
        console.error('Failed to fetch flows:', error)
      }
    }

    fetchFlows()
  }, [])

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Flows</h1>
          <p className="text-muted-foreground">
            Manage your workflow definitions
          </p>
        </div>
        <Button>
          <Plus className="mr-2 h-4 w-4" />
          Create Flow
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>All Flows</CardTitle>
          <CardDescription>
            A list of all registered flows in the system.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Created At</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {flows.map((flow) => (
                <TableRow key={flow.id}>
                  <TableCell className="font-medium">
                    <Link to={`/flows/${flow.id}`} className="hover:underline">
                      {flow.name}
                    </Link>
                  </TableCell>
                  <TableCell>
                    {new Date(flow.created_at * 1000).toLocaleString()}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button variant="ghost" size="sm">
                      Edit
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
              {flows.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={3}
                    className="text-center h-24 text-muted-foreground"
                  >
                    No flows found.
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
