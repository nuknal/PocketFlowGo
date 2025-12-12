# PocketFlowGo

PocketFlowGo is a lightweight, centralized task orchestration service written in Go. It lets you define business workflows as versioned graphs, execute tasks across remote workers over HTTP, and persist state and run history in SQLite for durability and recovery.

## Highlights
- Versioned workflow model: `flows` → `flow_versions` → `tasks`
- Rich node types: `executor`, `choice`, `parallel`, `subflow`, `timer`, `foreach`, `wait_event`, `approval`
- Remote workers over HTTP with registration, heartbeat, and load-aware allocation
- Durable state in SQLite: task cursor, shared state, retries, leases, and full node run logs
- Crash-safe scheduling loop that resumes leased tasks
- Async Queue (Pull Mode) for workers behind firewalls or long-running tasks

## Quick Start

Prerequisites: Go 1.20+

1) Start the Scheduler

```bash
SCHEDULER_DB=./scheduler.db go run cmd/scheduler/main.go
```

2) Start a Worker

```bash
REGISTRY_URL=http://localhost:8070 \
WORKER_URL=http://localhost:8080 \
go run cmd/worker/main.go
```

3) Run the CLI demo

```bash
SCHEDULER_BASE=http://localhost:8070 go run cmd/cli/main.go
```

This will create example flows and tasks, then poll until tasks complete. Check the console output for task status and node run details.

## Data Model
- `flows`: logical workflow namespace (id, name)
- `flow_versions`: concrete version with JSON graph definition and status (e.g., `published`)
- `tasks`: execution instance bound to a `flow_version` with runtime fields like `current_node_key`, `shared_json`, `lease_owner`, etc.

Relationship: one `flow` has many `flow_versions`; one `flow_version` is referenced by many `tasks`.

## Flow Definition (JSON)

Minimal example:

```json
{
  "start": "decide",
  "nodes": {
    "decide": {"kind": "executor", "service": "route", "post": {"action_key": "action"}},
    "B": {"kind": "executor", "service": "transform", "prep": {"input_key": "$params.text"}, "post": {"output_key": "result", "action_static": "default"}},
    "C": {"kind": "executor", "service": "transform", "prep": {"input_key": "$params.text"}, "post": {"output_key": "result", "action_static": "default"}}
  },
  "edges": [
    {"from": "decide", "action": "goB", "to": "B"},
    {"from": "decide", "action": "goC", "to": "C"},
    {"from": "B", "action": "default", "to": ""},
    {"from": "C", "action": "default", "to": ""}
  ]
}
```

Key fields:
- `start`: node key to begin execution
- `nodes`: map of node definitions with `kind`, `service`, `exec_type` (default `http`, optional `queue`), `prep`, `params`, and `post`
- `edges`: array of `{from, action, to}`; `action="default"` is the fallback

## Async Queue Mode (Pull)

For workers that cannot be reached directly via HTTP (e.g., behind firewalls) or for long-running tasks, use `exec_type: "queue"`.

1. **Define Node**: Set `"exec_type": "queue"` in the node definition.
2. **Enqueue**: The engine suspends the task and writes a job to the queue table.
3. **Poll**: Workers call `POST /queue/poll` to fetch pending jobs.
4. **Complete**: Workers process the job and call `POST /queue/complete` with the result.
5. **Resume**: The engine resumes the flow execution with the reported result.

## HTTP API

Worker Registry (compatible routes):
- `POST /workers/register`
- `POST /workers/heartbeat`
- `GET /workers/list?service=...`
- `GET /workers/allocate?service=...`

Flows & Versions:
- `POST /flows` → create flow
- `POST /flows/version` → create and publish version with definition JSON

Tasks:
- `POST /tasks` → create task referencing latest published version of a flow
- `GET /tasks?status=...` → list tasks
- `GET /tasks/get?id=...` → task details
- `POST /tasks/cancel?id=...` → mark as `canceling`
- `GET /tasks/runs?task_id=...` → node run log
- `POST /tasks/signal` → write a key/value into task shared state (for `wait_event/approval`)

Queue (Pull Mode):
- `POST /queue/poll` → worker polls for pending tasks
- `POST /queue/complete` → worker reports task completion with result

## Components & Layout
- `cmd/scheduler`: HTTP API + scheduling loop
- `cmd/worker`: HTTP worker providing services like `transform`, `sum`, `route`
- `cmd/cli`: loads demo flows, creates tasks, and polls for completion
- `internal/api`: HTTP handlers for flows, versions, tasks, workers
- `internal/engine`: execution engine for nodes and edges
- `internal/store`: SQLite schema and read/write operations

## Testing

Run unit tests:

```bash
go test ./...
```

Notable tests include engine integration scenarios and utility functions.

## Notes
- SQLite is used by default; the model can be extended to Postgres.
- Leases (`lease_owner/lease_expiry`) prevent double execution; expired leases are reclaimed.
- See `docs/architecture.md` for a detailed design record.
