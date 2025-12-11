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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Plus, Play } from 'lucide-react'
import { Link } from 'react-router-dom'
import { api, type Flow } from '@/lib/api'
import { RunTaskDialog } from '@/components/RunTaskDialog'

export default function Flows() {
  const [flows, setFlows] = useState<Flow[]>([])
  const [newFlowName, setNewFlowName] = useState('')
  const [newFlowDesc, setNewFlowDesc] = useState('')
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [runDialogFlow, setRunDialogFlow] = useState<{
    id: string
    name: string
  } | null>(null)

  const fetchFlows = async () => {
    try {
      const data = await api.getFlows()
      setFlows(data || [])
    } catch (error) {
      console.error('Failed to fetch flows:', error)
    }
  }

  useEffect(() => {
    fetchFlows()
  }, [])

  const handleCreateFlow = async () => {
    if (!newFlowName.trim()) return
    setCreating(true)
    try {
      await api.createFlow(newFlowName, newFlowDesc)
      setNewFlowName('')
      setNewFlowDesc('')
      setIsDialogOpen(false)
      fetchFlows()
    } catch (error) {
      console.error('Failed to create flow:', error)
    } finally {
      setCreating(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Flows</h1>
          <p className="text-muted-foreground">
            Manage your workflow definitions
          </p>
        </div>
        <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="mr-2 h-4 w-4" />
              Create Flow
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-[425px]">
            <DialogHeader>
              <DialogTitle>Create Flow</DialogTitle>
              <DialogDescription>
                Enter a name for the new flow.
              </DialogDescription>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="grid grid-cols-4 items-center gap-4">
                <Label htmlFor="name" className="text-right">
                  Name
                </Label>
                <Input
                  id="name"
                  value={newFlowName}
                  onChange={(e) => setNewFlowName(e.target.value)}
                  className="col-span-3"
                />
              </div>
              <div className="grid grid-cols-4 items-center gap-4">
                <Label htmlFor="description" className="text-right">
                  Description
                </Label>
                <Input
                  id="description"
                  value={newFlowDesc}
                  onChange={(e) => setNewFlowDesc(e.target.value)}
                  className="col-span-3"
                />
              </div>
            </div>
            <DialogFooter>
              <Button
                type="submit"
                onClick={handleCreateFlow}
                disabled={creating}
              >
                {creating ? 'Creating...' : 'Create'}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
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
                <TableHead>Description</TableHead>
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
                  <TableCell className="text-muted-foreground">
                    {flow.description || '-'}
                  </TableCell>
                  <TableCell>
                    {new Date(flow.created_at * 1000).toLocaleString()}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex items-center justify-end gap-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-8 px-2 lg:px-3"
                        onClick={() =>
                          setRunDialogFlow({ id: flow.id, name: flow.name })
                        }
                      >
                        <Play className="h-3.5 w-3.5 mr-1.5" />
                        Run
                      </Button>
                      <Link to={`/flows/${flow.id}`}>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-8 px-2 lg:px-3"
                        >
                          Edit
                        </Button>
                      </Link>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
              {flows.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={4}
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

      {runDialogFlow && (
        <RunTaskDialog
          open={!!runDialogFlow}
          onOpenChange={(open) => !open && setRunDialogFlow(null)}
          flowId={runDialogFlow.id}
          version={0} // 0 means latest version
          flowName={runDialogFlow.name}
          onSuccess={() => setRunDialogFlow(null)}
        />
      )}
    </div>
  )
}
