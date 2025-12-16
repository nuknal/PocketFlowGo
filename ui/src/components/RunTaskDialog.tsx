import { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'
import { useNavigate } from 'react-router-dom'
import { extractParamsFromDefinition } from '@/lib/flowUtils'

interface RunTaskDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  flowId: string
  version: number
  flowName?: string
  definition?: string
  onSuccess?: (taskId: string) => void
}
export function RunTaskDialog({
  open,
  onOpenChange,
  flowId,
  version,
  flowName,
  definition,
  onSuccess,
}: RunTaskDialogProps) {
  const [creating, setCreating] = useState(false)
  const [taskParams, setTaskParams] = useState('{}')
  const navigate = useNavigate()

  useEffect(() => {
    if (open && definition) {
      setTaskParams(extractParamsFromDefinition(definition))
    } else if (open) {
      setTaskParams('{}')
    }
  }, [open, definition])

  const handleCreateTask = async () => {
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

      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const { id } = await api.createTask(flowId, version, params)
      onOpenChange(false)
      setTaskParams('{}')
      if (onSuccess) {
        onSuccess(id)
      } else {
        // Default behavior: navigate to the new task
        navigate(`/tasks/${id}`)
      }
    } catch (error) {
      console.error('Failed to create task:', error)
      alert('Failed to create task')
    } finally {
      setCreating(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>Run Task</DialogTitle>
          <DialogDescription>
            Start a new execution for {flowName || 'Flow'} (v{version}).
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-4 py-4">
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
          <Button type="submit" onClick={handleCreateTask} disabled={creating}>
            {creating ? 'Starting...' : 'Start Execution'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
