package sqlstore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nuknal/PocketFlowGo/pkg/store"
)

// SQLite implements the Store interface using a SQLite database.
type SQLite struct{ DB *sql.DB }

// OpenSQLite opens a connection to the SQLite database at the given path.
func OpenSQLite(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	s := &SQLite{DB: db}
	if err := s.Init(); err != nil {
		return nil, err
	}
	return s, nil
}

// Init initializes the database schema.
func (s *SQLite) Init() error {
	stmts := []string{
		"PRAGMA foreign_keys = ON;",
		"CREATE TABLE IF NOT EXISTS flows (id TEXT PRIMARY KEY, name TEXT, description TEXT, created_at INTEGER);",
		"CREATE TABLE IF NOT EXISTS flow_versions (id TEXT PRIMARY KEY, flow_id TEXT, version INTEGER, definition_json TEXT, status TEXT, created_at INTEGER);",
		"CREATE TABLE IF NOT EXISTS nodes (id TEXT PRIMARY KEY, flow_version_id TEXT, node_key TEXT, service TEXT, kind TEXT, config_json TEXT);",
		"CREATE TABLE IF NOT EXISTS edges (id TEXT PRIMARY KEY, flow_version_id TEXT, from_node_key TEXT, action TEXT, to_node_key TEXT);",
		"CREATE TABLE IF NOT EXISTS tasks (id TEXT PRIMARY KEY, flow_version_id TEXT, status TEXT, params_json TEXT, shared_json TEXT, current_node_key TEXT, last_action TEXT, step_count INTEGER, retry_state_json TEXT, lease_owner TEXT, lease_expiry INTEGER, request_id TEXT, created_at INTEGER, updated_at INTEGER);",
		"CREATE TABLE IF NOT EXISTS node_runs (id TEXT PRIMARY KEY, task_id TEXT, node_key TEXT, attempt_no INTEGER, status TEXT, sub_status TEXT, branch_id TEXT, prep_json TEXT, exec_input_json TEXT, exec_output_json TEXT, error_text TEXT, action TEXT, started_at INTEGER, finished_at INTEGER, worker_id TEXT, worker_url TEXT, log_path TEXT);",
		"CREATE TABLE IF NOT EXISTS workers (id TEXT PRIMARY KEY, url TEXT, services_json TEXT, load INTEGER, last_heartbeat INTEGER, status TEXT, type TEXT);",
		"CREATE TABLE IF NOT EXISTS task_queue (id TEXT PRIMARY KEY, task_id TEXT, node_key TEXT, service TEXT, input_json TEXT, status TEXT, worker_id TEXT, created_at INTEGER, started_at INTEGER, timeout_at INTEGER);",
		"CREATE INDEX IF NOT EXISTS idx_queue_service_status ON task_queue(service, status);",
	}
	for _, q := range stmts {
		if _, err := s.DB.Exec(q); err != nil {
			return err
		}
	}
	// Try to add description column if it doesn't exist (simplistic migration)
	_, _ = s.DB.Exec("ALTER TABLE flows ADD COLUMN description TEXT")
	// Add type column to workers if it doesn't exist
	_, _ = s.DB.Exec("ALTER TABLE workers ADD COLUMN type TEXT")
	// Add sub_status and branch_id to node_runs
	_, _ = s.DB.Exec("ALTER TABLE node_runs ADD COLUMN sub_status TEXT")
	_, _ = s.DB.Exec("ALTER TABLE node_runs ADD COLUMN branch_id TEXT")
	_, _ = s.DB.Exec("ALTER TABLE node_runs ADD COLUMN log_path TEXT")
	return nil
}

func nowUnix() int64 { return store.NowUnix() }

func genID(prefix string) string { return store.GenID(prefix) }

// RegisterWorker registers or updates a worker in the database.
func (s *SQLite) RegisterWorker(w store.WorkerInfo) error {
	b, _ := json.Marshal(w.Services)
	// Default to http if type is missing
	if w.Type == "" {
		w.Type = "http"
	}
	_, err := s.DB.Exec("INSERT INTO workers(id,url,services_json,load,last_heartbeat,status,type) VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET url=excluded.url, services_json=excluded.services_json, load=excluded.load, last_heartbeat=excluded.last_heartbeat, status=excluded.status, type=excluded.type", w.ID, w.URL, string(b), w.Load, nowUnix(), w.Status, w.Type)
	return err
}

func (s *SQLite) HeartbeatWorker(id string, url string, load int) error {
	_, err := s.DB.Exec("UPDATE workers SET last_heartbeat=?, load=?, status='online' WHERE id=? OR url=?", nowUnix(), load, id, url)
	return err
}

func (s *SQLite) RefreshWorkersStatus(ttl int64) error {
	if ttl <= 0 {
		return nil
	}
	th := nowUnix() - ttl
	_, err := s.DB.Exec("UPDATE workers SET status='offline' WHERE last_heartbeat>0 AND last_heartbeat<?", th)
	return err
}

func (s *SQLite) ListWorkers(service string, ttl int64) ([]store.WorkerInfo, error) {
	rows, err := s.DB.Query("SELECT id,url,services_json,load,last_heartbeat,status,type FROM workers")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []store.WorkerInfo{}
	now := nowUnix()
	for rows.Next() {
		var id, url, sj, status string
		var typeStr sql.NullString
		var load int
		var hb int64
		if err := rows.Scan(&id, &url, &sj, &load, &hb, &status, &typeStr); err != nil {
			return nil, err
		}
		if ttl > 0 && now-hb > ttl {
			continue
		}
		var arr []string
		_ = json.Unmarshal([]byte(sj), &arr)
		if service != "" {
			ok := false
			for _, sname := range arr {
				if sname == service {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		out = append(out, store.WorkerInfo{ID: id, URL: url, Services: arr, Load: load, LastHeartbeat: hb, Status: status, Type: typeStr.String})
	}
	return out, nil
}

func (s *SQLite) CreateFlow(name string, description string) (string, error) {
	id := genID("flow")
	_, err := s.DB.Exec("INSERT INTO flows(id,name,description,created_at) VALUES(?,?,?,?)", id, name, description, nowUnix())
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *SQLite) CreateFlowVersion(flowID string, version int, definitionJSON string, status string) (string, error) {
	id := genID("ver")
	_, err := s.DB.Exec("INSERT INTO flow_versions(id,flow_id,version,definition_json,status,created_at) VALUES(?,?,?,?,?,?)", id, flowID, version, definitionJSON, status, nowUnix())
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *SQLite) ListFlows(limit, offset int) ([]store.Flow, int64, error) {
	var count int64
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM flows").Scan(&count); err != nil {
		return nil, 0, err
	}

	q := "SELECT id, name, description, created_at FROM flows ORDER BY created_at DESC"
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	}

	rows, err := s.DB.Query(q)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var flows []store.Flow
	for rows.Next() {
		var f store.Flow
		// Handle potential NULL description
		var desc sql.NullString
		if err := rows.Scan(&f.ID, &f.Name, &desc, &f.CreatedAt); err != nil {
			return nil, 0, err
		}
		f.Description = desc.String
		flows = append(flows, f)
	}
	return flows, count, nil
}

func (s *SQLite) ListFlowVersions(flowID string) ([]store.FlowVersion, error) {
	rows, err := s.DB.Query("SELECT id, flow_id, version, definition_json, status FROM flow_versions WHERE flow_id=? ORDER BY version DESC", flowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var versions []store.FlowVersion
	for rows.Next() {
		var fv store.FlowVersion
		if err := rows.Scan(&fv.ID, &fv.FlowID, &fv.Version, &fv.DefinitionJSON, &fv.Status); err != nil {
			return nil, err
		}
		versions = append(versions, fv)
	}
	return versions, nil
}

func (s *SQLite) LatestPublishedVersion(flowID string) (store.FlowVersion, error) {
	row := s.DB.QueryRow("SELECT id,flow_id,version,definition_json,status FROM flow_versions WHERE flow_id=? AND status='published' ORDER BY version DESC LIMIT 1", flowID)
	var fv store.FlowVersion
	if err := row.Scan(&fv.ID, &fv.FlowID, &fv.Version, &fv.DefinitionJSON, &fv.Status); err != nil {
		return store.FlowVersion{}, err
	}
	return fv, nil
}

func (s *SQLite) GetFlowVersionByFlowIDAndVersion(flowID string, version int) (store.FlowVersion, error) {
	row := s.DB.QueryRow("SELECT id,flow_id,version,definition_json,status FROM flow_versions WHERE flow_id=? AND version=?", flowID, version)
	var fv store.FlowVersion
	if err := row.Scan(&fv.ID, &fv.FlowID, &fv.Version, &fv.DefinitionJSON, &fv.Status); err != nil {
		return store.FlowVersion{}, err
	}
	return fv, nil
}

func (s *SQLite) GetFlowVersionByID(id string) (store.FlowVersion, error) {
	row := s.DB.QueryRow("SELECT id,flow_id,version,definition_json,status FROM flow_versions WHERE id=?", id)
	var fv store.FlowVersion
	if err := row.Scan(&fv.ID, &fv.FlowID, &fv.Version, &fv.DefinitionJSON, &fv.Status); err != nil {
		return store.FlowVersion{}, err
	}
	return fv, nil
}

func (s *SQLite) CreateTask(flowVersionID string, paramsJSON string, requestID string, startNode string) (string, error) {
	id := genID("task")
	_, err := s.DB.Exec("INSERT INTO tasks(id,flow_version_id,status,params_json,shared_json,current_node_key,last_action,step_count,retry_state_json,lease_owner,lease_expiry,request_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)", id, flowVersionID, "pending", paramsJSON, "{}", startNode, "", 0, "{}", "", 0, requestID, nowUnix(), nowUnix())
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *SQLite) GetTask(id string) (store.Task, error) {
	q := `SELECT 
		t.id, t.flow_version_id, t.status, t.params_json, t.shared_json, t.current_node_key, t.last_action, t.step_count, t.retry_state_json, t.lease_owner, t.lease_expiry, t.request_id, t.created_at, t.updated_at,
		COALESCE(f.id, ''), COALESCE(f.name, ''), COALESCE(fv.version, 0)
	FROM tasks t
	LEFT JOIN flow_versions fv ON t.flow_version_id = fv.id
	LEFT JOIN flows f ON fv.flow_id = f.id
	WHERE t.id=?`
	row := s.DB.QueryRow(q, id)
	var t store.Task
	if err := row.Scan(&t.ID, &t.FlowVersionID, &t.Status, &t.ParamsJSON, &t.SharedJSON, &t.CurrentNodeKey, &t.LastAction, &t.StepCount, &t.RetryStateJSON, &t.LeaseOwner, &t.LeaseExpiry, &t.RequestID, &t.CreatedAt, &t.UpdatedAt, &t.FlowID, &t.FlowName, &t.FlowVersion); err != nil {
		return store.Task{}, err
	}
	return t, nil
}

func (s *SQLite) LeaseNextTask(owner string, ttlSec int64) (store.Task, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return store.Task{}, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	now := nowUnix()
	row := tx.QueryRow("SELECT id FROM tasks WHERE status IN ('pending','running') AND (lease_expiry=0 OR lease_expiry<?) ORDER BY updated_at ASC LIMIT 1", now)
	var id string
	if err = row.Scan(&id); err != nil {
		return store.Task{}, err
	}
	res, uerr := tx.Exec("UPDATE tasks SET lease_owner=?, lease_expiry=?, status='running' WHERE id=? AND (lease_expiry=0 OR lease_expiry<?)", owner, now+ttlSec, id, now)
	if uerr != nil {
		return store.Task{}, uerr
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return store.Task{}, fmt.Errorf("lease_conflict")
	}
	return s.GetTask(id)
}

func (s *SQLite) ExtendLease(id string, owner string, ttlSec int64) error {
	_, err := s.DB.Exec("UPDATE tasks SET lease_owner=?, lease_expiry=? WHERE id=? AND lease_owner=?", owner, nowUnix()+ttlSec, id, owner)
	return err
}

func (s *SQLite) UpdateTaskStatus(id string, status string) error {
	_, err := s.DB.Exec("UPDATE tasks SET status=?, updated_at=? WHERE id=?", status, nowUnix(), id)
	return err
}

func (s *SQLite) UpdateTaskStatusOwned(id string, owner string, status string) error {
	_, err := s.DB.Exec("UPDATE tasks SET status=?, updated_at=? WHERE id=? AND lease_owner=? AND lease_expiry>?", status, nowUnix(), id, owner, nowUnix())
	return err
}

func (s *SQLite) SaveNodeRun(nr map[string]interface{}) error {
	id := genID("run")
	nr["id"] = id
	return s.CreateNodeRun(nr)
}

func (s *SQLite) CreateNodeRun(nr map[string]interface{}) error {
	cols := []string{"id", "task_id", "node_key", "attempt_no", "status", "sub_status", "branch_id", "prep_json", "exec_input_json", "exec_output_json", "error_text", "action", "started_at", "finished_at", "worker_id", "worker_url", "log_path"}
	vals := make([]interface{}, 0, len(cols))
	for _, c := range cols {
		if val, ok := nr[c]; ok {
			vals = append(vals, val)
		} else {
			vals = append(vals, nil)
		}
	}
	ph := strings.Repeat("?,", len(cols))
	ph = ph[:len(ph)-1]
	_, err := s.DB.Exec("INSERT INTO node_runs("+strings.Join(cols, ",")+") VALUES("+ph+")", vals...)
	return err
}

func (s *SQLite) UpdateNodeRun(id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	cols := []string{}
	vals := []interface{}{}
	for k, v := range updates {
		cols = append(cols, fmt.Sprintf("%s=?", k))
		vals = append(vals, v)
	}
	vals = append(vals, id)
	q := fmt.Sprintf("UPDATE node_runs SET %s WHERE id=?", strings.Join(cols, ","))
	_, err := s.DB.Exec(q, vals...)
	return err
}

func (s *SQLite) UpdateTaskProgress(id string, currentNode string, lastAction string, sharedJSON string, stepCount int) error {
	_, err := s.DB.Exec("UPDATE tasks SET current_node_key=?, last_action=?, shared_json=?, step_count=?, updated_at=? WHERE id=?", currentNode, lastAction, sharedJSON, stepCount, nowUnix(), id)
	return err
}

func (s *SQLite) UpdateTaskProgressOwned(id string, owner string, currentNode string, lastAction string, sharedJSON string, stepCount int) error {
	_, err := s.DB.Exec("UPDATE tasks SET current_node_key=?, last_action=?, shared_json=?, step_count=?, updated_at=? WHERE id=? AND lease_owner=? AND lease_expiry>?", currentNode, lastAction, sharedJSON, stepCount, nowUnix(), id, owner, nowUnix())
	return err
}

func (s *SQLite) ListTasks(status string, flowVersionID string, limit, offset int) ([]store.Task, int64, error) {
	// Count query
	countQ := `SELECT COUNT(*) FROM tasks t WHERE 1=1`
	countArgs := []interface{}{}
	if status != "" {
		countQ += " AND t.status=?"
		countArgs = append(countArgs, status)
	}
	if flowVersionID != "" {
		countQ += " AND t.flow_version_id=?"
		countArgs = append(countArgs, flowVersionID)
	}
	var count int64
	if err := s.DB.QueryRow(countQ, countArgs...).Scan(&count); err != nil {
		return nil, 0, err
	}

	q := `SELECT 
		t.id, t.flow_version_id, t.status, t.params_json, t.shared_json, t.current_node_key, t.last_action, t.step_count, t.retry_state_json, t.lease_owner, t.lease_expiry, t.request_id, t.created_at, t.updated_at,
		COALESCE(f.id, ''), COALESCE(f.name, ''), COALESCE(fv.version, 0)
	FROM tasks t
	LEFT JOIN flow_versions fv ON t.flow_version_id = fv.id
	LEFT JOIN flows f ON fv.flow_id = f.id
	WHERE 1=1`
	args := []interface{}{}
	if status != "" {
		q += " AND t.status=?"
		args = append(args, status)
	}
	if flowVersionID != "" {
		q += " AND t.flow_version_id=?"
		args = append(args, flowVersionID)
	}
	q += " ORDER BY t.updated_at DESC"
	if limit > 0 {
		q += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []store.Task{}
	for rows.Next() {
		var t store.Task
		if err := rows.Scan(&t.ID, &t.FlowVersionID, &t.Status, &t.ParamsJSON, &t.SharedJSON, &t.CurrentNodeKey, &t.LastAction, &t.StepCount, &t.RetryStateJSON, &t.LeaseOwner, &t.LeaseExpiry, &t.RequestID, &t.CreatedAt, &t.UpdatedAt, &t.FlowID, &t.FlowName, &t.FlowVersion); err != nil {
			return nil, 0, err
		}
		out = append(out, t)
	}
	return out, count, nil
}

func (s *SQLite) ListNodeRuns(taskID string) ([]store.NodeRun, error) {
	rows, err := s.DB.Query("SELECT id,task_id,node_key,attempt_no,status,sub_status,branch_id,prep_json,exec_input_json,exec_output_json,error_text,action,started_at,finished_at,worker_id,worker_url,log_path FROM node_runs WHERE task_id=? ORDER BY started_at ASC", taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []store.NodeRun{}
	for rows.Next() {
		var r store.NodeRun
		var sub, branch, lp sql.NullString
		if err := rows.Scan(&r.ID, &r.TaskID, &r.NodeKey, &r.AttemptNo, &r.Status, &sub, &branch, &r.PrepJSON, &r.ExecInputJSON, &r.ExecOutputJSON, &r.ErrorText, &r.Action, &r.StartedAt, &r.FinishedAt, &r.WorkerID, &r.WorkerURL, &lp); err != nil {
			return nil, err
		}
		r.SubStatus = sub.String
		r.BranchID = branch.String
		r.LogPath = lp.String
		out = append(out, r)
	}
	return out, nil
}

func (s *SQLite) GetNodeRun(id string) (store.NodeRun, error) {
	row := s.DB.QueryRow("SELECT id,task_id,node_key,attempt_no,status,sub_status,branch_id,prep_json,exec_input_json,exec_output_json,error_text,action,started_at,finished_at,worker_id,worker_url,log_path FROM node_runs WHERE id=?", id)
	var r store.NodeRun
	var sub, branch, lp sql.NullString
	if err := row.Scan(&r.ID, &r.TaskID, &r.NodeKey, &r.AttemptNo, &r.Status, &sub, &branch, &r.PrepJSON, &r.ExecInputJSON, &r.ExecOutputJSON, &r.ErrorText, &r.Action, &r.StartedAt, &r.FinishedAt, &r.WorkerID, &r.WorkerURL, &lp); err != nil {
		return store.NodeRun{}, err
	}
	r.SubStatus = sub.String
	r.BranchID = branch.String
	r.LogPath = lp.String
	return r, nil
}

// EnqueueTask adds a new task to the queue
func (s *SQLite) EnqueueTask(taskID, nodeKey, service, inputJSON string) (string, error) {
	id := genID("q")
	_, err := s.DB.Exec("INSERT INTO task_queue(id,task_id,node_key,service,input_json,status,worker_id,created_at,started_at,timeout_at) VALUES(?,?,?,?,?,?,?,?,?,?)",
		id, taskID, nodeKey, service, inputJSON, "pending", "", nowUnix(), 0, 0)
	if err != nil {
		return "", err
	}
	return id, nil
}

// PollQueue attempts to claim a pending task for the given services
func (s *SQLite) PollQueue(workerID string, services []string, timeoutSec int64) (store.QueueTask, error) {
	if len(services) == 0 {
		return store.QueueTask{}, nil
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return store.QueueTask{}, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	placeholders := strings.Repeat("?,", len(services))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]interface{}, len(services))
	for i, svc := range services {
		args[i] = svc
	}

	// Find oldest pending task matching services
	q := fmt.Sprintf("SELECT id, task_id, node_key, service, input_json FROM task_queue WHERE status='pending' AND service IN (%s) ORDER BY created_at ASC LIMIT 1", placeholders)

	var qt store.QueueTask
	if err := tx.QueryRow(q, args...).Scan(&qt.ID, &qt.TaskID, &qt.NodeKey, &qt.Service, &qt.InputJSON); err != nil {
		if err == sql.ErrNoRows {
			return store.QueueTask{}, nil
		}
		return store.QueueTask{}, err
	}

	// Claim it
	now := nowUnix()
	timeoutAt := now + timeoutSec
	res, err := tx.Exec("UPDATE task_queue SET status='claimed', worker_id=?, started_at=?, timeout_at=? WHERE id=? AND status='pending'",
		workerID, now, timeoutAt, qt.ID)
	if err != nil {
		return store.QueueTask{}, err
	}

	aff, _ := res.RowsAffected()
	if aff == 0 {
		// Race condition: someone else claimed it
		return store.QueueTask{}, nil
	}

	qt.Status = "claimed"
	qt.WorkerID = workerID
	qt.StartedAt = now
	qt.TimeoutAt = timeoutAt

	return qt, nil
}

// CompleteQueueTask marks a queue task as completed (or failed)
// Note: The actual result/error is stored in node_runs via a separate call, or we could add result columns here.
// For now, we assume the engine will handle the result, but typically the worker reports result here.
// To keep it simple, we'll just mark status here, and let the caller handle the data persistence elsewhere or add columns if needed.
// Wait, the design says Worker calls /queue/complete with result. So we need to return the TaskID so the API can update the flow.
func (s *SQLite) CompleteQueueTask(queueID string) (string, error) {
	var taskID string
	err := s.DB.QueryRow("SELECT task_id FROM task_queue WHERE id=?", queueID).Scan(&taskID)
	if err != nil {
		return "", err
	}

	_, err = s.DB.Exec("UPDATE task_queue SET status='completed' WHERE id=?", queueID)
	if err != nil {
		return "", err
	}
	return taskID, nil
}

// FailQueueTask marks a queue task as failed
func (s *SQLite) FailQueueTask(queueID string) error {
	_, err := s.DB.Exec("UPDATE task_queue SET status='failed' WHERE id=?", queueID)
	return err
}
