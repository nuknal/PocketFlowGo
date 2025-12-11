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
		"CREATE TABLE IF NOT EXISTS flows (id TEXT PRIMARY KEY, name TEXT, description TEXT, created_at INTEGER);",
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
	// Try to add description column if it doesn't exist (simplistic migration)
	_, _ = s.DB.Exec("ALTER TABLE flows ADD COLUMN description TEXT")
	return nil
}

func nowUnix() int64 { return time.Now().Unix() }

func genID(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }

type WorkerInfo struct {
	ID            string   `json:"id"`
	URL           string   `json:"url"`
	Services      []string `json:"services"`
	Load          int      `json:"load"`
	LastHeartbeat int64    `json:"last_heartbeat"`
	Status        string   `json:"status"`
}

func (s *SQLite) RegisterWorker(w WorkerInfo) error {
	b, _ := json.Marshal(w.Services)
	_, err := s.DB.Exec("INSERT INTO workers(id,url,services_json,load,last_heartbeat,status) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET url=excluded.url, services_json=excluded.services_json, load=excluded.load, last_heartbeat=excluded.last_heartbeat, status=excluded.status", w.ID, w.URL, string(b), w.Load, nowUnix(), w.Status)
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
		out = append(out, WorkerInfo{ID: id, URL: url, Services: arr, Load: load, LastHeartbeat: hb, Status: status})
	}
	return out, nil
}

type Flow struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   int64  `json:"created_at"`
}
type FlowVersion struct {
	ID             string `json:"id"`
	FlowID         string `json:"flow_id"`
	Version        int    `json:"version"`
	DefinitionJSON string `json:"definition_json"`
	Status         string `json:"status"`
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

func (s *SQLite) ListFlows() ([]Flow, error) {
	rows, err := s.DB.Query("SELECT id, name, description, created_at FROM flows ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var flows []Flow
	for rows.Next() {
		var f Flow
		// Handle potential NULL description
		var desc sql.NullString
		if err := rows.Scan(&f.ID, &f.Name, &desc, &f.CreatedAt); err != nil {
			return nil, err
		}
		f.Description = desc.String
		flows = append(flows, f)
	}
	return flows, nil
}

func (s *SQLite) ListFlowVersions(flowID string) ([]FlowVersion, error) {
	rows, err := s.DB.Query("SELECT id, flow_id, version, definition_json, status FROM flow_versions WHERE flow_id=? ORDER BY version DESC", flowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var versions []FlowVersion
	for rows.Next() {
		var fv FlowVersion
		if err := rows.Scan(&fv.ID, &fv.FlowID, &fv.Version, &fv.DefinitionJSON, &fv.Status); err != nil {
			return nil, err
		}
		versions = append(versions, fv)
	}
	return versions, nil
}

func (s *SQLite) LatestPublishedVersion(flowID string) (FlowVersion, error) {
	row := s.DB.QueryRow("SELECT id,flow_id,version,definition_json,status FROM flow_versions WHERE flow_id=? AND status='published' ORDER BY version DESC LIMIT 1", flowID)
	var fv FlowVersion
	if err := row.Scan(&fv.ID, &fv.FlowID, &fv.Version, &fv.DefinitionJSON, &fv.Status); err != nil {
		return FlowVersion{}, err
	}
	return fv, nil
}

func (s *SQLite) GetFlowVersionByFlowIDAndVersion(flowID string, version int) (FlowVersion, error) {
	row := s.DB.QueryRow("SELECT id,flow_id,version,definition_json,status FROM flow_versions WHERE flow_id=? AND version=?", flowID, version)
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
	ID             string `json:"id"`
	FlowVersionID  string `json:"flow_version_id"`
	FlowID         string `json:"flow_id,omitempty"`
	FlowName       string `json:"flow_name,omitempty"`
	FlowVersion    int    `json:"flow_version,omitempty"`
	Status         string `json:"status"`
	ParamsJSON     string `json:"params_json"`
	SharedJSON     string `json:"shared_json"`
	CurrentNodeKey string `json:"current_node_key"`
	LastAction     string `json:"last_action"`
	StepCount      int    `json:"step_count"`
	RetryStateJSON string `json:"retry_state_json"`
	LeaseOwner     string `json:"lease_owner"`
	LeaseExpiry    int64  `json:"lease_expiry"`
	RequestID      string `json:"request_id"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
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
	q := `SELECT 
		t.id, t.flow_version_id, t.status, t.params_json, t.shared_json, t.current_node_key, t.last_action, t.step_count, t.retry_state_json, t.lease_owner, t.lease_expiry, t.request_id, t.created_at, t.updated_at,
		COALESCE(f.id, ''), COALESCE(f.name, ''), COALESCE(fv.version, 0)
	FROM tasks t
	LEFT JOIN flow_versions fv ON t.flow_version_id = fv.id
	LEFT JOIN flows f ON fv.flow_id = f.id
	WHERE t.id=?`
	row := s.DB.QueryRow(q, id)
	var t Task
	if err := row.Scan(&t.ID, &t.FlowVersionID, &t.Status, &t.ParamsJSON, &t.SharedJSON, &t.CurrentNodeKey, &t.LastAction, &t.StepCount, &t.RetryStateJSON, &t.LeaseOwner, &t.LeaseExpiry, &t.RequestID, &t.CreatedAt, &t.UpdatedAt, &t.FlowID, &t.FlowName, &t.FlowVersion); err != nil {
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
	now := nowUnix()
	row := tx.QueryRow("SELECT id FROM tasks WHERE status IN ('pending','running') AND (lease_expiry=0 OR lease_expiry<?) ORDER BY updated_at ASC LIMIT 1", now)
	var id string
	if err = row.Scan(&id); err != nil {
		return Task{}, err
	}
	res, uerr := tx.Exec("UPDATE tasks SET lease_owner=?, lease_expiry=?, status='running' WHERE id=? AND (lease_expiry=0 OR lease_expiry<?)", owner, now+ttlSec, id, now)
	if uerr != nil {
		return Task{}, uerr
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return Task{}, fmt.Errorf("lease_conflict")
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

func (s *SQLite) UpdateTaskProgressOwned(id string, owner string, currentNode string, lastAction string, sharedJSON string, stepCount int) error {
	_, err := s.DB.Exec("UPDATE tasks SET current_node_key=?, last_action=?, shared_json=?, step_count=?, updated_at=? WHERE id=? AND lease_owner=? AND lease_expiry>?", currentNode, lastAction, sharedJSON, stepCount, nowUnix(), id, owner, nowUnix())
	return err
}

func (s *SQLite) ListTasks(status string, flowVersionID string, limit int) ([]Task, error) {
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
		if err := rows.Scan(&t.ID, &t.FlowVersionID, &t.Status, &t.ParamsJSON, &t.SharedJSON, &t.CurrentNodeKey, &t.LastAction, &t.StepCount, &t.RetryStateJSON, &t.LeaseOwner, &t.LeaseExpiry, &t.RequestID, &t.CreatedAt, &t.UpdatedAt, &t.FlowID, &t.FlowName, &t.FlowVersion); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

type NodeRun struct {
	ID             string `json:"id"`
	TaskID         string `json:"task_id"`
	NodeKey        string `json:"node_key"`
	AttemptNo      int    `json:"attempt_no"`
	Status         string `json:"status"`
	PrepJSON       string `json:"prep_json"`
	ExecInputJSON  string `json:"exec_input_json"`
	ExecOutputJSON string `json:"exec_output_json"`
	ErrorText      string `json:"error_text"`
	Action         string `json:"action"`
	StartedAt      int64  `json:"started_at"`
	FinishedAt     int64  `json:"finished_at"`
	WorkerID       string `json:"worker_id"`
	WorkerURL      string `json:"worker_url"`
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
