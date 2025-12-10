# PocketFlowGo Scheduler: Design & Development Notes

## Goals & Scope

- Upgrade PocketFlowGo from in-memory scheduling to a centralized orchestration service with flow composition, task management, node run logging, worker registry/heartbeat, query and auto-advancing capabilities.
- Support crash recovery: after service restarts, task state and run history are restored from SQLite.
- Keep dependencies minimal: SQLite (extensible to Postgres), Go standard library + light drivers; reuse existing remote node protocol.

## Architecture Overview

- Components
  - Scheduler: HTTP API + scheduling loop + Engine
  - Store (SQLite): table schema and read/write operations
  - Engine: reads version definition JSON, executes node `prep/exec/post`, picks edges, and advances tasks
  - Worker: remote execution service (`/exec/<service>`), registers and heartbeats
  - CLI: creates Flow/Version/Task, polls results, prints node run details

- Data flow
  1. Client submits Flow and Version (definition JSON) to the scheduler
  2. Client creates Task (referencing latest published version), persisted as `pending`
  3. Scheduling loop leases tasks, Engine advances nodes, calls Workers, writes `node_runs` and task cursor
  4. Upon completion marks task `completed`, query shared state and run history

## Data Model (SQLite)

- `flows`: `id,name,created_at`
- `flow_versions`: `id,flow_id,version,definition_json,status,created_at`
- `tasks`:
  - `id,flow_version_id,status(pending|running|completed|failed|canceling|canceled),params_json,shared_json`
  - `current_node_key,last_action,step_count,retry_state_json,lease_owner,lease_expiry,request_id,created_at,updated_at`
- `node_runs`:
  - `id,task_id,node_key,attempt_no,status(ok|error|canceled),prep_json,exec_input_json,exec_output_json,error_text,action,started_at,finished_at,worker_id,worker_url`
- `workers`: `id,url,services_json,load,last_heartbeat,status`

References: `internal/store/sqlite.go`

## Flow Definition (FlowDef JSON)

- Structure
  - `start`: starting node key
  - `nodes`: `key -> DefNode`
    - `kind`: `executor | choice | parallel | subflow | timer | foreach | wait_event | approval`
    - `service`: remote service name (Worker route)
    - `params`: node params, merged into task params and passed to Worker
    - `prep.input_key`: input path; supports shared keys or `$params.<key>` prefix
    - `prep.input_map`: batch mapping `{toKey: fromPath}`; `fromPath` supports `$params.` prefix
    - `post.output_key`: write execution result into shared state
    - `post.output_map`: batch copy `{toKey: fromField}` from result into shared state
    - `post.action_static | post.action_key`: fixed action or extract from result
    - Retry/switching: `max_retries, wait_ms, max_attempts, attempt_delay_ms, weighted_by_load`
  - `edges`: `{from, action, to}`; `action='default'` denotes the fallback edge

References: `internal/engine/types.go:38-42`

## HTTP API

- Worker Registry (compatible routes)
  - `POST /workers/register`
  - `POST /workers/heartbeat`
  - `GET /workers/list?service=...&ttl=...`
  - `GET /workers/allocate?service=...`
- Flows & Versions
  - `POST /flows` → create Flow
  - `POST /flows/version` → create and publish Version
- Tasks
  - `POST /tasks` → create Task using latest published Version of a Flow
  - `GET /tasks?status=...` → list
  - `GET /tasks/get?id=...` → details (including shared state)
  - `POST /tasks/cancel?id=...` → mark as `canceling`
  - `GET /tasks/runs?task_id=...` → node run history
  - `POST /tasks/signal` → write key/value into task shared state (for `wait_event/approval`)

References: `internal/api/server.go:93-127,130-174,215-223,225-259`

## Engine (Advance Once)

- Input: task `id`
- Steps:
  1. Read task and corresponding version JSON
  2. Parse current node, merge params, build input via `prep` (supports `$params.` prefixes)
  3. Call Worker (optionally sorted by load; failure switch controlled by `max_attempts/attempt_delay_ms`)
  4. Node-level retries (`max_retries/wait_ms`), each attempt writes to `node_runs`
  5. On success, write shared state and action; choose edge, update cursor and status
  6. On failure with no successor edge, mark task `failed`
  7. If task is `canceling`, mark `canceled` and record a run

References: `internal/engine/core.go:112-158`, `internal/engine/executor.go:1-53`

## Scheduling Loop & Leases

- Loop: background goroutine leases next task, then keeps advancing it to completion or no successor; extend lease before each step.
- Lease strategy: fields `lease_owner/lease_expiry` avoid duplicate execution; SQLite uses lease instead of row locks.

References: `cmd/scheduler/main.go:33-51`, `internal/store/sqlite.go:186-214`

## Worker Protocol & Implementation

- Protocol: `POST /exec/<service>`; body: `{"input":..., "params":{...}}`; returns `{"result":..., "error":""}`
- Services:
  - `transform`: `upper/lower/mul`
  - `sum`: sum array
  - `route`: returns `{action: goB|goC}` based on params
- Load & heartbeat: atomic during processing; heartbeat carries `load`; scheduler updates Worker load; allocation may sort by load.
- Port binding: derives port from `WORKER_URL`, falls back to random if conflict; registers with actual bind address.

References: `cmd/worker/main.go`

## CLI Demo

- Behavior: create Flow/Version (with branches), create tasks for B/C branches, poll to completion, print results and node run details.
- Usage: `SCHEDULER_BASE=http://localhost:8070 go run cmd/cli/main.go`

References: `cmd/cli/main.go`

## Configuration & Run

- Start scheduler: `SCHEDULER_DB=./scheduler.db go run cmd/scheduler/main.go`
- Start worker: `REGISTRY_URL=http://localhost:8070 WORKER_URL=http://localhost:8080 go run cmd/worker/main.go`
- Run CLI: `SCHEDULER_BASE=http://localhost:8070 go run cmd/cli/main.go`

## Operations & Recovery

- TTL filtering: Workers exceeding heartbeat TTL are excluded from allocation and listing.
- Background offline refresh: scheduler periodically marks stale Workers `offline`.
  - Env: `WORKER_OFFLINE_TTL_SEC` (default `15`), `WORKER_REFRESH_INTERVAL_SEC` (default `5`)
  - Impl: `cmd/scheduler/main.go:25-31,33-51`, `internal/store/sqlite.go:217-229`
- Crash recovery: scheduling loop reclaims expired leases of `running` tasks and continues advancing.
- Audit: `/tasks/runs` returns node run history for diagnostics and metrics.

## Node Types & Configuration

- Common fields
  - `kind`: node type (`executor | choice | parallel | subflow | timer | foreach | wait_event | approval`)
  - `params`: node params merged with task params
  - `prep.input_key` / `prep.input_map`: input selection from `$params/$shared/$input`
  - `post.output_key` / `post.output_map`: write result(s) to shared state
  - `post.action_static` / `post.action_key`: action selection
  - `default_action`: used by `choice` when no case matches or no `post.action_*`
  - Retry/switch: `max_retries, wait_ms, max_attempts, attempt_delay_ms, weighted_by_load`

- Executor (`kind: executor`)
  - `service`: remote service name
  - Input/output per common fields
  - Action: prefer `post.action_static`, else `post.action_key`
  - Failure: if no successor edge and failure, task marked `failed`

- Choice (`kind: choice`)
  - `choice_cases`: array of `{action, expr}`; first match wins
  - Expr ops: `and | or | not | eq | ne | gt | lt | ge | le | exists | in | contains`; paths support `$params/$shared/$input`
  - Output: write `prep` input to `post.output_key` if set
  - Fallback: `post.action_key` or `default_action`

- Parallel (`kind: parallel`)
  - `parallel_services`: static list or derived from `params.services` (string array)
  - `parallel_mode`: `sequential | concurrent` (default `sequential`)
  - `max_parallel`: cap concurrent batch size
  - `failure_strategy`: `fail_fast | collect_errors | ignore_errors`
  - Aggregation: after completion, write ordered results array into `post.output_key`
  - Runtime: `_rt.pl:<nodeKey>` keeps `{done, errs, mode, max, strategy}`

- Subflow (`kind: subflow`)
  - `subflow`: embedded flow, same structure as `FlowDef`
  - Runtime: `_rt.sf:<nodeKey>` keeps `{curr, shared, last}`; `shared` is subflow internal shared state
  - Advance: on completion, write subflow `shared` into parent’s `post.output_key`
  - Action: determined by parent node’s `post.action_*`

- Timer (`kind: timer`)
  - `params.delay_ms`: delay in ms; `post.action_static` action after due
  - Runtime: `_rt.tm:<nodeKey>` keeps `{start}` (ms timestamp)

- Foreach (`kind: foreach`)
  - Input: `prep.input_key` (array)
  - Service: `service` invoked per item
  - Concurrency: `parallel_mode`, `max_parallel`
  - Failure policy: `failure_strategy`
  - Aggregation: writes result array to `post.output_key`, selects action via `post.action_*`
  - Runtime: `_rt.fe:<nodeKey>` keeps `{done, errs, idx, mode, max, strategy}`

- Wait Event (`kind: wait_event`)
  - `params.signal_key`: resolve from `$shared/$params/$input`
  - `params.timeout_ms`: optional timeout
  - Action: `post.action_static|action_key`
  - Runtime: `_rt.we:<nodeKey>` keeps `{start}`

- Approval (`kind: approval`)
  - `params.approval_key`: resolve from `$shared/$params/$input`
  - `post.action_key`: from approval value, or boolean/strings map to `approved|rejected`
  - Runtime: `_rt.ap:<nodeKey>`

References:
- Node types & structs: `internal/engine/types.go:38-42`
- Dispatch entry: `internal/engine/core.go:112-158`
- Executor: `internal/engine/executor.go:1-53`
- Parallel: `internal/engine/parallel.go:1-163`
- Subflow: `internal/engine/subflow.go:1-118`
- Choice: `internal/engine/choice.go:1-38`
- Expression eval: `internal/engine/expr.go:1-200`
- Timer: `internal/engine/timer.go`
- Foreach: `internal/engine/foreach.go`
- Wait event: `internal/engine/wait_event.go:1-88`
- Approval: `internal/engine/approval.go:1-61`

