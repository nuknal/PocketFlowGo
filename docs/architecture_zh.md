# PocketFlowGo 任务调度中心：设计与开发记录

## 目标与范围

- 将 PocketFlowGo 从“本地内存调度”升级为“中心化任务调度服务”，提供流程编排、任务管理、节点执行记录、Worker 注册与心跳、查询与自动推进能力。
- 支持故障恢复：服务重启后任务状态与运行轨迹均可从数据库恢复。
- 保持依赖最小：使用 SQLite（可扩展至 Postgres），标准库 + 轻量驱动；复用现有远程节点协议。

## 架构概览

- 组件
  - 调度中心（Scheduler）：HTTP API + 调度循环 + 引擎（Engine）
  - 存储层（Store/SQLite）：表结构与读写事务
  - 引擎（Engine）：读取版本定义 JSON，执行节点 `prep/exec/post`，选择边并推进任务
  - Worker：远程执行服务（`/exec/<service>`），注册与心跳到调度中心
  - CLI：一键创建 Flow/Version/Task，轮询打印结果与节点运行明细

- 关键数据流
  1. 客户端提交 Flow 与 Version（定义图 JSON）到调度中心
  2. 客户端创建 Task（引用最新发布版本），持久化为 `pending`
  3. 调度循环抢占任务（租约），引擎按图推进节点，调用 Worker，写回 `node_runs` 与任务游标
  4. 任务完成后标记 `completed`，可查询共享态与运行轨迹

## 数据模型（SQLite）

- flows：`id,name,created_at`
- flow_versions：`id,flow_id,version,definition_json,status,created_at`
- tasks：
  - `id,flow_version_id,status(pending|running|completed|failed|canceling|canceled),params_json,shared_json`
  - `current_node_key,last_action,step_count,retry_state_json,lease_owner,lease_expiry,request_id,created_at,updated_at`
- node_runs：
  - `id,task_id,node_key,attempt_no,status(ok|error|canceled),prep_json,exec_input_json,exec_output_json,error_text,action,started_at,finished_at,worker_id,worker_url`
- workers：`id,url,services_json,load,last_heartbeat,status`

实现参考：`internal/store/sqlite.go`（创建与读写）

## 版本定义（FlowDef JSON）

- 结构
  - `start`: 起始节点键
  - `nodes`: `key -> DefNode`
    - `kind`: `executor|local|flow_ref`（当前使用 `executor`）
    - `service`: 远程服务名（Worker 路由名）
    - `params`: 节点参数，合并到任务参数中传递给 Worker
    - `prep.input_key`: 输入获取键；支持共享态键或 `$params.<key>` 前缀从任务参数取值
    - `post.output_key`: 将执行结果写回共享态的键
    - `post.action_static|post.action_key`: 固定动作或从执行结果按键提取动作
    - 重试/切换：`max_retries, wait_ms, max_attempts, attempt_delay_ms, weighted_by_load`
  - `edges`: `from,action,to`，`action='default'` 表示默认边

实现参考：`internal/engine/types.go:3-30`（结构体定义）

## 调度中心 HTTP API

- Worker 注册中心（与旧路径兼容）
  - `POST /workers/register`（兼容 `/register`）
  - `POST /workers/heartbeat`（兼容 `/heartbeat`）
  - `GET /workers/list?service=...`（兼容 `/list`）
  - `GET /workers/allocate?service=...`（兼容 `/allocate`）
- Flow 与 Version
  - `POST /flows`：创建 Flow
  - `POST /flows/version`：创建并发布版本定义（含图 JSON）
- Task 管理
  - `POST /tasks`：创建任务，抓取最新发布版本并使用其 `start`
  - `GET /tasks?status=...`：列表查询
  - `GET /tasks/get?id=...`：任务详情（含共享态 JSON）
  - `POST /tasks/cancel?id=...`：置为 `canceling`
  - `GET /tasks/runs?task_id=...`：节点运行明细列表

实现参考：`internal/api/server.go`

## 引擎（推进一次）

- 输入：任务 `id`
- 过程：
  1. 读取任务与对应版本定义 JSON
  2. 解析当前节点，合并参数，`prep` 取输入（支持 `$params.` 前缀）
  3. 调用 Worker（按负载排序，失败切换受 `max_attempts/attempt_delay_ms` 控制）
  4. 节点级重试（`max_retries/wait_ms`），每次写入 `node_runs`
  5. 成功则写回共享态与动作，选择边得到 `next`，更新任务游标与状态
  6. 失败且无后继边则标记 `failed`
  7. 若任务为 `canceling`，直接标记 `canceled` 并记录一条运行

实现参考：`internal/engine/core.go:114-152`（`RunOnce` 分发），`internal/engine/remote.go:14-43`（执行节点）

## 自动调度循环与租约

- 调度循环：后台 goroutine 轮询 `LeaseNextTask` 抢占任务，随后持续推进至完成或无后继；每步前 `ExtendLease` 续租。
- 抢占策略：租约字段 `lease_owner/lease_expiry`，避免双跑；SQLite 下使用租约代替行锁。

实现参考：`cmd/scheduler/main.go`

## Worker 协议与实现

- 协议：`POST /exec/<service>`，Body：`{"input":..., "params":{...}}`，返回：`{"result":..., "error":""}`
- 现有服务：
  - `transform`：`upper/lower/mul`
  - `sum`：对数组求和
  - `route`：根据参数 `action` 返回 `{action: goB|goC}`
- 负载与心跳：处理期间使用原子计数；心跳携带 `load`，调度中心更新 Worker 负载；调度时可按负载排序。
- 端口绑定：从 `WORKER_URL` 解析端口，冲突时随机回退，并以实际绑定地址完成注册。

实现参考：`cmd/worker/main.go`

## CLI 演示

- 行为：创建 Flow/Version（含条件分支），创建两条任务（走 B/C 分支），轮询至完成，打印结果与节点运行明细。
- 使用：`SCHEDULER_BASE=http://localhost:8070 go run cmd/cli/main.go`

实现参考：`cmd/cli/main.go`

## 配置与运行

- 启动调度中心：`SCHEDULER_DB=./scheduler.db go run cmd/scheduler/main.go`
- 启动 Worker：`REGISTRY_URL=http://localhost:8070 WORKER_URL=http://localhost:8080 go run cmd/worker/main.go`
- 运行 CLI：`SCHEDULER_BASE=http://localhost:8070 go run cmd/cli/main.go`

## 运维与恢复

- TTL 过滤：心跳超时的 Worker 不参与分配
- 崩溃恢复：调度循环抢占过期租约的 `running` 任务并继续推进
- 审计查询：`/tasks/runs` 返回节点运行轨迹，便于问题定位与统计

## 下一步迭代建议

- 节点执行超时与错误映射到动作（保护分支/回退）
- 并行批：将批次拆为多任务，并引入汇总节点聚合结果（避免共享态并发写冲突）
- 取消识别全链路：推进前检查 `canceling`，节点端可感知取消并提前结束
- Postgres 存储与 `SKIP LOCKED` 抢占简化
- Web 管理页面：任务列表、详情、运行轨迹、Worker 状态面板
## 节点类型与配置详解

- 通用字段（所有节点适用）
  - `kind`: 节点类型（`executor | choice | parallel | subflow | timer | foreach | wait_event | approval`）
  - `params`: 节点级参数，运行时与任务参数合并后传递给执行端
  - `prep.input_key`: 从共享态或任务参数取输入；支持 `$params.<key>` 前缀
  - `prep.input_map`: 批量映射输入，`{toKey: fromPath}`，`fromPath` 支持 `$params.` 前缀
  - `post.output_key`: 将执行结果写入共享态该键（或聚合结果/子流共享态）
  - `post.output_map`: 批量从结果中拷贝字段到共享态，`{toKey: fromField}`
  - `post.action_static`: 固定动作名，用于选择后继边
  - `post.action_key`: 从执行结果或共享态按键提取动作名
  - `default_action`: 在 `choice` 无命中或无 `post.action_*` 时使用
  - 重试与切换：
    - `max_retries`: 节点级重试次数（同一 Worker）
    - `wait_ms`: 节点级重试的等待毫秒数
    - `max_attempts`: Worker 切换尝试上限（跨 Worker）
    - `attempt_delay_ms`: Worker 切换失败后的等待毫秒数
    - `weighted_by_load`: 是否按 Worker 负载排序选择

- 执行节点（`kind: executor`）
  - `service`: 执行的远程服务名（Worker 路由名）
  - 输入与输出：遵循通用 `prep/post` 字段
  - 动作选择：优先 `post.action_static`，其次 `post.action_key`（从结果取值）
  - 失败处理：无后继边且失败则任务标记为 `failed`

- 选择节点（`kind: choice`）
  - `choice_cases`: 数组，每项 `{action, expr}`；按表达式求值命中第一个则采用其 `action`
  - `expr` 支持：`and | or | not | eq | ne | gt | lt | ge | le | exists | in | contains`，路径支持 `$params./$shared./$input`
  - 输出：若设置 `post.output_key`，将 `prep` 输入写回该键
  - 回退：若未命中任何 `choice_cases`，按 `post.action_key` 或 `default_action`

- 并行节点（`kind: parallel`）
  - `parallel_services`: 固定服务列表；若未设置可从 `params.services` 读取（字符串数组）
  - `parallel_mode`: `sequential | concurrent`（默认 `sequential`）
  - `max_parallel`: 并发模式下一批启动上限（<= 剩余数）
  - `failure_strategy`: `fail_fast | collect_errors | ignore_errors`
    - `fail_fast`: 本批有错即结束并聚合已有成功结果，推进后继
    - `collect_errors`: 全部分支完成后聚合结果与错误
    - `ignore_errors`: 忽略错误分支，仅聚合成功结果
  - 聚合：完成后将各分支结果按顺序聚合为数组写入 `post.output_key`
  - 运行态：内部在共享态 `_rt.pl:<nodeKey>` 存储 `{done, errs, mode, max, strategy}`

- 子流节点（`kind: subflow`）
  - `subflow`: 嵌入式子图，结构同 `FlowDef`（含 `start/nodes/edges`）
  - 共享态：维护 `_rt.sf:<nodeKey>`，包含 `{curr, shared, last}`；`shared` 为子流内部共享态
  - 推进：子流完成后将其 `shared` 写入父流的 `post.output_key`
  - 动作选择：按父节点的 `post.action_*` 决定父流后继边

- 边（`edges`）
  - 结构：`{from, action, to}`
  - 默认边：当 `action` 为空或未匹配时，使用 `action='default'` 的边

实现参考：
- 节点类型与结构：`internal/engine/types.go:3-30`
- 入口分发：`internal/engine/core.go:114-152`
- 执行节点：`internal/engine/remote.go:14-43`、`internal/engine/remote.go:46-88`
- 并行节点：`internal/engine/parallel.go:1-163`
- 子流节点：`internal/engine/subflow.go:1-118`
- 选择节点：`internal/engine/choice.go:1-40`
- 表达式求值：`internal/engine/expr.go:1-200`

## 高优先级复杂节点

- 定时器节点（`kind: timer`）
  - 作用：延时推进后继边
  - 配置：`params.delay_ms`（毫秒），`post.action_static`（到期后动作）。可选 `post.output_key` 写入输入
  - 运行态：`_rt.tm:<nodeKey>` 保存 `{start}`（毫秒时间戳）
  - 示例：
    ```
    {"kind":"timer","params":{"delay_ms":3000},"post":{"action_static":"next"}}
    ```

- 遍历节点（`kind: foreach`）
  - 作用：对集合逐项执行远程服务，支持顺序与并发，聚合结果
  - 配置：
    - 输入集合：`prep.input_key`（数组）
    - 执行服务：`service`（每项作为输入调用该服务）
    - 并发控制：`parallel_mode: sequential|concurrent`，`max_parallel`
    - 失败策略：`failure_strategy: fail_fast | collect_errors | ignore_errors`
    - 结果聚合：`post.output_key` 写入结果数组；动作 `post.action_*`
  - 运行态：`_rt.fe:<nodeKey>` 保存 `{done, errs, idx, mode, max, strategy}`
  - 示例：
    ```
    {"kind":"foreach","service":"transform","prep":{"input_key":"$shared.items"},"params":{"mul":2.0},"post":{"output_key":"mapped","action_static":"next"},"parallel_mode":"concurrent","max_parallel":8}
    ```

- 事件等待节点（`kind: wait_event`）
  - 作用：等待外部事件/信号就绪，再推进；可配置超时
  - 配置：`params.signal_key`（从 `$shared/$params/$input` 解析），`params.timeout_ms`，`post.action_static|action_key`
  - 运行态：`_rt.we:<nodeKey>` 保存 `{start}`（毫秒时间戳）
  - 示例：
    ```
    {"kind":"wait_event","params":{"signal_key":"$shared.flag","timeout_ms":60000},"post":{"action_static":"next"}}
    ```

- 审批节点（`kind: approval`）
  - 作用：人机协作，等待审批结果后推进
  - 配置：`params.approval_key`（从 `$shared/$params/$input` 解析），`post.action_key` 或按布尔/字符串映射 `approved|rejected`
  - 运行态：`_rt.ap:<nodeKey>`
  - 示例：
    ```
    {"kind":"approval","params":{"approval_key":"$shared.approval"},"post":{"action_key":"approval"}}
    ```

实现参考：
- 定时器：`internal/engine/timer.go:1-51`
- 遍历：`internal/engine/foreach.go:1-105`
- 事件等待：`internal/engine/wait_event.go:1-46`
- 审批：`internal/engine/approval.go:1-45`
