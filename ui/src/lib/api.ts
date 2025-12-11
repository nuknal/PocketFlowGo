export const API_BASE_URL =
  import.meta.env.VITE_API_BASE_URL || 'http://localhost:8070'

export interface Worker {
  id: string
  url: string
  services: string[]
  load: number
  last_heartbeat: number
  status: string
}

export interface Task {
  id: string
  flow_version_id: string
  flow_id?: string
  flow_name?: string
  flow_version?: number
  status: string
  params_json: string
  shared_json: string
  current_node_key: string
  last_action: string
  step_count: number
  retry_state_json: string
  lease_owner: string
  lease_expiry: number
  request_id: string
  created_at: number
  updated_at: number
}

export interface Flow {
  id: string
  name: string
  description: string
  created_at: number
}

export interface NodeRun {
  id: number
  task_id: string
  node_key: string
  attempt_no: number
  status: string
  prep_json: string
  exec_input_json: string
  exec_output_json: string
  error_text: string
  action: string
  started_at: number
  finished_at: number
  worker_id: string
  worker_url: string
}

export interface FlowVersion {
  id: string
  flow_id: string
  version: number
  definition_json: string
  status: string
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  size: number
}

export const api = {
  getWorkers: async (service?: string, ttl?: number): Promise<Worker[]> => {
    const params = new URLSearchParams()
    if (service) params.append('service', service)
    if (ttl) params.append('ttl', ttl.toString())
    const res = await fetch(`${API_BASE_URL}/workers/list?${params.toString()}`)
    if (!res.ok) throw new Error('Failed to fetch workers')
    return res.json()
  },

  getTasks: async (
    status?: string,
    flowVersionId?: string,
    page: number = 1,
    pageSize: number = 10
  ): Promise<PaginatedResponse<Task>> => {
    const params = new URLSearchParams()
    if (status && status !== 'all') params.append('status', status)
    if (flowVersionId) params.append('flow_version_id', flowVersionId)
    params.append('page', page.toString())
    params.append('page_size', pageSize.toString())
    const res = await fetch(`${API_BASE_URL}/tasks?${params.toString()}`)
    if (!res.ok) throw new Error('Failed to fetch tasks')
    return res.json()
  },

  getFlowVersion: async (id: string): Promise<FlowVersion> => {
    const res = await fetch(`${API_BASE_URL}/flows/version/get?id=${id}`)
    if (!res.ok) throw new Error('Failed to fetch flow version')
    return res.json()
  },

  getTask: async (id: string): Promise<Task> => {
    const res = await fetch(`${API_BASE_URL}/tasks/get?id=${id}`)
    if (!res.ok) throw new Error('Failed to fetch task')
    return res.json()
  },

  createTask: async (
    flowId: string,
    version: number = 0,
    params: any = {}
  ): Promise<{ id: string }> => {
    const res = await fetch(`${API_BASE_URL}/tasks`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        FlowID: flowId,
        Version: version,
        ParamsJSON: JSON.stringify(params),
      }),
    })
    if (!res.ok) throw new Error('Failed to create task')
    return res.json()
  },

  getTaskRuns: async (taskId: string): Promise<NodeRun[]> => {
    const res = await fetch(`${API_BASE_URL}/tasks/runs?task_id=${taskId}`)
    if (!res.ok) throw new Error('Failed to fetch task runs')
    return res.json()
  },

  getFlows: async (
    page: number = 1,
    pageSize: number = 10
  ): Promise<PaginatedResponse<Flow>> => {
    const params = new URLSearchParams()
    params.append('page', page.toString())
    params.append('page_size', pageSize.toString())
    const res = await fetch(`${API_BASE_URL}/flows?${params.toString()}`)
    if (!res.ok) throw new Error('Failed to fetch flows')
    return res.json()
  },

  getFlowVersions: async (flowId: string): Promise<FlowVersion[]> => {
    const res = await fetch(`${API_BASE_URL}/flows/version?flow_id=${flowId}`)
    if (!res.ok) throw new Error('Failed to fetch flow versions')
    return res.json()
  },

  createFlow: async (
    name: string,
    description: string = ''
  ): Promise<{ id: string }> => {
    const res = await fetch(`${API_BASE_URL}/flows`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ Name: name, Description: description }),
    })
    if (!res.ok) throw new Error('Failed to create flow')
    return res.json()
  },

  createFlowVersion: async (
    flowId: string,
    version: number,
    definition: string,
    status: string = 'published'
  ): Promise<{ id: string }> => {
    const res = await fetch(`${API_BASE_URL}/flows/version`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        FlowID: flowId,
        Version: version,
        DefinitionJSON: definition,
        Status: status,
      }),
    })
    if (!res.ok) throw new Error('Failed to create flow version')
    return res.json()
  },
}
