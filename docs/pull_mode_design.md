# PocketFlowGo: Pull Mode Design

## 1. Overview
Current architecture uses a **Push Mode** where the Engine directly calls Workers via HTTP. To support internal network workers, better flow control, and decoupling, we will introduce a **Pull Mode**.

In Pull Mode:
1. Engine writes execution requests to a persistent **Queue** (DB table).
2. Workers actively **Poll** the queue for tasks.
3. Workers execute tasks and report results back via **Complete** API.

## 2. Data Model (SQLite)

We will introduce a new table `task_queue` to act as the broker.

```sql
CREATE TABLE task_queue (
    id TEXT PRIMARY KEY,          -- Unique ID for this execution attempt
    task_id TEXT,                 -- Reference to the parent Flow Task
    node_key TEXT,                -- Which node is being executed
    service TEXT,                 -- Target service name (e.g. "image-processor")
    input_json TEXT,              -- Payload for the worker
    status TEXT,                  -- pending | claimed | completed | failed | timeout
    worker_id TEXT,               -- ID of the worker who claimed the task
    created_at INTEGER,           -- Timestamp when enqueued
    started_at INTEGER,           -- Timestamp when claimed
    timeout_at INTEGER            -- When the task should be considered timed out
);

-- Index for efficient polling
CREATE INDEX idx_queue_service_status ON task_queue(service, status);
```

## 3. API Changes

### 3.1 Worker Polling API
`POST /queue/poll`
- **Request**: `{"worker_id": "w-123", "services": ["image-resize", "email"]}`
- **Response**: 
  - 200 OK: `{"id": "q-999", "service": "image-resize", "input": {...}, "token": "..."}`
  - 204 No Content: (If no tasks available)

### 3.2 Task Completion API
`POST /queue/complete`
- **Request**: `{"queue_id": "q-999", "result": {...}, "error": null}`
- **Response**: 200 OK

### 3.3 Heartbeat API (Optional for V1)
`POST /queue/heartbeat`
- To extend `timeout_at` for long-running tasks.

## 4. Engine Architecture Changes

### 4.1 "Async" Executor
We need a new executor logic or a modification to `ExecutorHTTP`.
Let's call it `ExecutorQueue`.

**Behavior:**
1. **Enqueue**: Instead of calling HTTP, insert a row into `task_queue` with `status='pending'`.
2. **Suspend**: The Engine must **stop** processing the current Flow Task.
   - We need a new Task status: `WAITING_QUEUE` (or reuse `RUNNING` but with a specific internal state).
   - In the current loop-based Engine, we can return a special error `ErrAsyncPending` or similar signal to break the loop without marking the task as failed.

### 4.2 Async Completion Handler
We need a background process or a callback hook.
When `/queue/complete` is called:
1. Update `task_queue` row to `completed`.
2. Update the parent `tasks` row:
   - Load the Flow Context.
   - Update Shared State with the result.
   - Advance the cursor (`current_node` -> `next_node`).
   - Change Task status back to `PENDING` (so the Scheduler picks it up again).

## 5. Worker Architecture Changes

The Worker binary needs a new mode:
- **Server Mode**: (Current) HTTP Server listening for requests.
- **Client Mode**: (New) Long-polling loop.

**Client Loop:**
```go
for {
    task := poll(services)
    if task == nil {
        sleep(100ms)
        continue
    }
    result, err := execute(task.Input)
    reportCompletion(task.ID, result, err)
}
```

## 6. Migration Strategy

1.  **Phase 1**: Implement `task_queue` table and API endpoints.
2.  **Phase 2**: Implement `ExecutorQueue` in Engine. Add a config flag `exec_mode: "queue"` to `DefNode`.
3.  **Phase 3**: Update Worker CLI to support polling mode.

## 7. Future Improvements
- **Long Polling**: Use HTTP Long Polling to reduce DB queries from idle workers.
- **Visibility Timeout**: If a worker crashes, the task should become visible to others after N seconds.
