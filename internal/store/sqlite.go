package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type SQLite struct{ DB *sql.DB }

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

func (s *SQLite) Init() error {
	stmts := []string{
		"PRAGMA foreign_keys = ON;",
		"CREATE TABLE IF NOT EXISTS flows (id TEXT PRIMARY KEY, name TEXT, created_at INTEGER);",
		"CREATE TABLE IF NOT EXISTS flow_versions (id TEXT PRIMARY KEY, flow_id TEXT, version INTEGER, definition_json TEXT, status TEXT, created_at INTEGER);",
		"CREATE TABLE IF NOT EXISTS nodes (id TEXT PRIMARY KEY, flow_version_id TEXT, node_key TEXT, service TEXT, kind TEXT, config_json TEXT);",
		"CREATE TABLE IF NOT EXISTS edges (id TEXT PRIMARY KEY, flow_version_id TEXT, from_node_key TEXT, action TEXT, to_node_key TEXT);",
		"CREATE TABLE IF NOT EXISTS tasks (id TEXT PRIMARY KEY, flow_version_id TEXT, status TEXT, params_json TEXT, shared_json TEXT, current_node_key TEXT, last_action TEXT, step_count INTEGER, retry_state_json TEXT, lease_owner TEXT, lease_expiry INTEGER, request_id TEXT, created_at INTEGER, updated_at INTEGER);",
		"CREATE TABLE IF NOT EXISTS node_runs (id TEXT PRIMARY KEY, task_id TEXT, node_key TEXT, attempt_no INTEGER, status TEXT, prep_json TEXT, exec_input_json TEXT, exec_output_json TEXT, error_text TEXT, action TEXT, started_at INTEGER, finished_at INTEGER, worker_id TEXT, worker_url TEXT);",
		"CREATE TABLE IF NOT EXISTS workers (id TEXT PRIMARY KEY, url TEXT, services_json TEXT, load INTEGER, last_heartbeat INTEGER, status TEXT);",
	}
	for _, q := range stmts {
		if _, err := s.DB.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func nowUnix() int64 { return time.Now().Unix() }

func genID(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }

type WorkerInfo struct {
	ID            string
	URL           string
	Services      []string
	Load          int
	LastHeartbeat int64
	Status        string
}

func (s *SQLite) RegisterWorker(w WorkerInfo) error {
	b, _ := json.Marshal(w.Services)
	_, err := s.DB.Exec("INSERT INTO workers(id,url,services_json,load,last_heartbeat,status) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET url=excluded.url, services_json=excluded.services_json, load=excluded.load, last_heartbeat=excluded.last_heartbeat, status=excluded.status", w.ID, w.URL, string(b), w.Load, nowUnix(), w.Status)
	return err
}

func (s *SQLite) HeartbeatWorker(id string, url string, load int) error {
	_, err := s.DB.Exec("UPDATE workers SET last_heartbeat=?, load=? WHERE id=? OR url=?", nowUnix(), load, id, url)
	return err
}

func (s *SQLite) ListWorkers(service string, ttl int64) ([]WorkerInfo, error) {
	rows, err := s.DB.Query("SELECT id,url,services_json,load,last_heartbeat,status FROM workers")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WorkerInfo{}
	now := nowUnix()
	for rows.Next() {
		var id, url, sj, status string
		var load int
		var hb int64
		if err := rows.Scan(&id, &url, &sj, &load, &hb, &status); err != nil {
			return nil, err
		}
		if ttl > 0 && now-hb > ttl {
			continue
		}
		var arr []string
		_ = json.Unmarshal([]byte(sj), &arr)
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
		out = append(out, WorkerInfo{ID: id, URL: url, Services: arr, Load: load, LastHeartbeat: hb, Status: status})
	}
	return out, nil
}

type Flow struct {
	ID   string
	Name string
}
type FlowVersion struct {
	ID             string
	FlowID         string
	Version        int
	DefinitionJSON string
	Status         string
}

func (s *SQLite) CreateFlow(name string) (string, error) {
	id := genID("flow")
	_, err := s.DB.Exec("INSERT INTO flows(id,name,created_at) VALUES(?,?,?)", id, name, nowUnix())
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

func (s *SQLite) LatestPublishedVersion(flowID string) (FlowVersion, error) {
	row := s.DB.QueryRow("SELECT id,flow_id,version,definition_json,status FROM flow_versions WHERE flow_id=? AND status='published' ORDER BY version DESC LIMIT 1", flowID)
	var fv FlowVersion
	if err := row.Scan(&fv.ID, &fv.FlowID, &fv.Version, &fv.DefinitionJSON, &fv.Status); err != nil {
		return FlowVersion{}, err
	}
	return fv, nil
}

func (s *SQLite) GetFlowVersionByID(id string) (FlowVersion, error) {
	row := s.DB.QueryRow("SELECT id,flow_id,version,definition_json,status FROM flow_versions WHERE id=?", id)
	var fv FlowVersion
	if err := row.Scan(&fv.ID, &fv.FlowID, &fv.Version, &fv.DefinitionJSON, &fv.Status); err != nil {
		return FlowVersion{}, err
	}
	return fv, nil
}

type Task struct {
	ID             string
	FlowVersionID  string
	Status         string
	ParamsJSON     string
	SharedJSON     string
	CurrentNodeKey string
	LastAction     string
	StepCount      int
	RetryStateJSON string
	LeaseOwner     string
	LeaseExpiry    int64
	RequestID      string
	CreatedAt      int64
	UpdatedAt      int64
}

func (s *SQLite) CreateTask(flowVersionID string, paramsJSON string, requestID string, startNode string) (string, error) {
	id := genID("task")
	_, err := s.DB.Exec("INSERT INTO tasks(id,flow_version_id,status,params_json,shared_json,current_node_key,last_action,step_count,retry_state_json,lease_owner,lease_expiry,request_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)", id, flowVersionID, "pending", paramsJSON, "{}", startNode, "", 0, "{}", "", 0, requestID, nowUnix(), nowUnix())
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *SQLite) GetTask(id string) (Task, error) {
	row := s.DB.QueryRow("SELECT id,flow_version_id,status,params_json,shared_json,current_node_key,last_action,step_count,retry_state_json,lease_owner,lease_expiry,request_id,created_at,updated_at FROM tasks WHERE id=?", id)
	var t Task
	if err := row.Scan(&t.ID, &t.FlowVersionID, &t.Status, &t.ParamsJSON, &t.SharedJSON, &t.CurrentNodeKey, &t.LastAction, &t.StepCount, &t.RetryStateJSON, &t.LeaseOwner, &t.LeaseExpiry, &t.RequestID, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return Task{}, err
	}
	return t, nil
}

func (s *SQLite) LeaseNextTask(owner string, ttlSec int64) (Task, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return Task{}, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()
	row := tx.QueryRow("SELECT id FROM tasks WHERE status IN ('pending','running') AND (lease_expiry=0 OR lease_expiry<?) LIMIT 1", nowUnix())
	var id string
	if err = row.Scan(&id); err != nil {
		return Task{}, err
	}
	_, err = tx.Exec("UPDATE tasks SET lease_owner=?, lease_expiry=?, status='running' WHERE id=?", owner, nowUnix()+ttlSec, id)
	if err != nil {
		return Task{}, err
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

func (s *SQLite) SaveNodeRun(nr map[string]interface{}) error {
	id := genID("run")
	nr["id"] = id
	cols := []string{"id", "task_id", "node_key", "attempt_no", "status", "prep_json", "exec_input_json", "exec_output_json", "error_text", "action", "started_at", "finished_at", "worker_id", "worker_url"}
	vals := make([]interface{}, 0, len(cols))
	for _, c := range cols {
		vals = append(vals, nr[c])
	}
	ph := strings.Repeat("?,", len(cols))
	ph = ph[:len(ph)-1]
	_, err := s.DB.Exec("INSERT INTO node_runs("+strings.Join(cols, ",")+") VALUES("+ph+")", vals...)
	return err
}

func (s *SQLite) UpdateTaskProgress(id string, currentNode string, lastAction string, sharedJSON string, stepCount int) error {
	_, err := s.DB.Exec("UPDATE tasks SET current_node_key=?, last_action=?, shared_json=?, step_count=?, updated_at=? WHERE id=?", currentNode, lastAction, sharedJSON, stepCount, nowUnix(), id)
	return err
}

func (s *SQLite) ListTasks(status string, limit int) ([]Task, error) {
	q := "SELECT id,flow_version_id,status,params_json,shared_json,current_node_key,last_action,step_count,retry_state_json,lease_owner,lease_expiry,request_id,created_at,updated_at FROM tasks"
	args := []interface{}{}
	if status != "" {
		q += " WHERE status=?"
		args = append(args, status)
	}
	q += " ORDER BY updated_at DESC"
	if limit <= 0 {
		limit = 100
	}
	q += " LIMIT ?"
	args = append(args, limit)
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.FlowVersionID, &t.Status, &t.ParamsJSON, &t.SharedJSON, &t.CurrentNodeKey, &t.LastAction, &t.StepCount, &t.RetryStateJSON, &t.LeaseOwner, &t.LeaseExpiry, &t.RequestID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

type NodeRun struct {
	ID             string `json:"id"`
	TaskID         string `json:"taskId"`
	NodeKey        string `json:"nodeKey"`
	AttemptNo      int    `json:"attemptNo"`
	Status         string `json:"status"`
	PrepJSON       string `json:"prepJson"`
	ExecInputJSON  string `json:"execInputJson"`
	ExecOutputJSON string `json:"execOutputJson"`
	ErrorText      string `json:"errorText"`
	Action         string `json:"action"`
	StartedAt      int64  `json:"startedAt"`
	FinishedAt     int64  `json:"finishedAt"`
	WorkerID       string `json:"workerId"`
	WorkerURL      string `json:"workerUrl"`
}

func (s *SQLite) ListNodeRuns(taskID string) ([]NodeRun, error) {
	rows, err := s.DB.Query("SELECT id,task_id,node_key,attempt_no,status,prep_json,exec_input_json,exec_output_json,error_text,action,started_at,finished_at,worker_id,worker_url FROM node_runs WHERE task_id=? ORDER BY started_at ASC", taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NodeRun{}
	for rows.Next() {
		var r NodeRun
		if err := rows.Scan(&r.ID, &r.TaskID, &r.NodeKey, &r.AttemptNo, &r.Status, &r.PrepJSON, &r.ExecInputJSON, &r.ExecOutputJSON, &r.ErrorText, &r.Action, &r.StartedAt, &r.FinishedAt, &r.WorkerID, &r.WorkerURL); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}
