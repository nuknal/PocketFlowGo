// Package api provides the HTTP API server for PocketFlowGo.
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/nuknal/PocketFlowGo/internal/engine"
	"github.com/nuknal/PocketFlowGo/internal/store"
)

// Server serves the API endpoints.
type Server struct{ Store *store.SQLite }

func writeJSON(w http.ResponseWriter, v interface{}, code int) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func withCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		h(w, r)
	}
}

// RegisterRoutes registers all API routes on the provided mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/workers/register", withCORS(s.handleRegisterWorker))
	mux.HandleFunc("/api/workers/heartbeat", withCORS(s.handleHeartbeat))
	mux.HandleFunc("/api/workers/list", withCORS(s.handleListWorkers))
	mux.HandleFunc("/api/workers/allocate", withCORS(s.handleAllocate))
	mux.HandleFunc("/api/register", withCORS(s.handleRegisterWorker))
	mux.HandleFunc("/api/heartbeat", withCORS(s.handleHeartbeat))
	mux.HandleFunc("/api/list", withCORS(s.handleListWorkers))
	mux.HandleFunc("/api/allocate", withCORS(s.handleAllocate))
	mux.HandleFunc("/api/flows", withCORS(s.handleFlows))
	mux.HandleFunc("/api/flows/version", withCORS(s.handleFlowVersion))
	mux.HandleFunc("/api/flows/version/get", withCORS(s.handleGetFlowVersion))
	mux.HandleFunc("/api/tasks", withCORS(s.handleTasks))
	mux.HandleFunc("/api/tasks/get", withCORS(s.handleGetTask))
	mux.HandleFunc("/api/tasks/run_once", withCORS(s.handleRunOnce))
	mux.HandleFunc("/api/tasks/cancel", withCORS(s.handleCancel))
	mux.HandleFunc("/api/tasks/runs", withCORS(s.handleTaskRuns))
	mux.HandleFunc("/api/tasks/signal", withCORS(s.handleTaskSignal))
	mux.HandleFunc("/api/queue/poll", withCORS(s.handleQueuePoll))
	mux.HandleFunc("/api/queue/complete", withCORS(s.handleQueueComplete))
}

func (s *Server) handleQueuePoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, map[string]string{"error": "method not allowed"}, 405)
		return
	}
	var payload struct {
		WorkerID string   `json:"worker_id"`
		Services []string `json:"services"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, map[string]string{"error": "bad request"}, 400)
		return
	}

	task, err := s.Store.PollQueue(payload.WorkerID, payload.Services, 60) // 60s visibility timeout
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()}, 500)
		return
	}
	if task.ID == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, task, 200)
}

func (s *Server) handleQueueComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, map[string]string{"error": "method not allowed"}, 405)
		return
	}
	var payload struct {
		QueueID string      `json:"queue_id"`
		Result  interface{} `json:"result"`
		Error   string      `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, map[string]string{"error": "bad request"}, 400)
		return
	}

	// 1. Mark queue task as completed
	taskID, err := s.Store.CompleteQueueTask(payload.QueueID)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()}, 500)
		return
	}

	// 2. Fetch queue task details to know node_key (optional optimization: return node_key from CompleteQueueTask)
	// For now, let's just use the engine to "Complete" the step.
	// But wait, the engine state is "WAITING_QUEUE". We need to wake it up.
	// The standard way is to record the run result and update the task status to PENDING so the scheduler picks it up.

	// We need to fetch the task to get context
	t, err := s.Store.GetTask(taskID)
	if err != nil {
		writeJSON(w, map[string]string{"error": "task not found"}, 404)
		return
	}

	// 3. Record the run
	// Note: We need the node_key. We can get it from the queue task if we had it, or from the task current_node.
	// Let's assume queue task corresponds to current_node_key.

	runStatus := "ok"
	if payload.Error != "" {
		runStatus = "error"
	}

	// We need to properly record the run.
	// Using a simple map for now as per SaveNodeRun signature
	run := map[string]interface{}{
		"task_id":          t.ID,
		"node_key":         t.CurrentNodeKey,
		"attempt_no":       1, // Simplified for now
		"status":           runStatus,
		"prep_json":        "{}", // We don't have prep info here easily without reloading flow def
		"exec_input_json":  "{}", // We don't have input here easily
		"exec_output_json": "{}", // We will store result below
		"error_text":       payload.Error,
		"action":           "",
		"started_at":       nowUnix(), // Approximate
		"finished_at":      nowUnix(),
		"worker_id":        "queue-worker", // We could track actual worker ID from payload if passed
		"worker_url":       "queue",
	}

	if payload.Result != nil {
		b, _ := json.Marshal(payload.Result)
		run["exec_output_json"] = string(b)
	}

	if err := s.Store.SaveNodeRun(run); err != nil {
		// Log error but continue
	}

	// 4. Update task status to PENDING so the scheduler picks it up and advances it
	// BUT: The scheduler needs to know that this "PENDING" means "Resume from Queue Result".
	// Currently, the engine re-executes the node if it's pending.
	// We need a way to tell the engine "Skip execution, I have the result".
	// A common pattern is to check if there's a successful run for the current node.

	// Let's set it to PENDING. The Engine logic needs to be smart enough to see "Oh, I have a completed run for this node, let me just process output and move on."
	// OR: We handle the transition right here (complex).
	// OR: We introduce a new status "RESUMING".

	// For this iteration, let's just set it to PENDING.
	// We will modify the Engine to check for completed runs before executing.
	if err := s.Store.UpdateTaskStatus(taskID, "pending"); err != nil {
		writeJSON(w, map[string]string{"error": err.Error()}, 500)
		return
	}

	writeJSON(w, map[string]string{"ok": "1"}, 200)
}

func nowUnix() int64 { return time.Now().Unix() }

func (s *Server) handleRegisterWorker(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ID       string
		URL      string
		Services []string
	}
	dec := json.NewDecoder(r.Body)
	_ = dec.Decode(&payload)
	_ = s.Store.RegisterWorker(store.WorkerInfo{ID: payload.ID, URL: payload.URL, Services: payload.Services, Load: 0, LastHeartbeat: time.Now().Unix(), Status: "online"})
	writeJSON(w, map[string]string{"ok": "1"}, 200)
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ID   string
		URL  string
		Load int
	}
	dec := json.NewDecoder(r.Body)
	_ = dec.Decode(&payload)
	_ = s.Store.HeartbeatWorker(payload.ID, payload.URL, payload.Load)
	writeJSON(w, map[string]string{"ok": "1"}, 200)
}

func (s *Server) handleListWorkers(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	ttlS := r.URL.Query().Get("ttl")
	ttl, _ := strconv.ParseInt(ttlS, 10, 64)
	if ttl == 0 {
		ttl = 15
	}
	_ = s.Store.RefreshWorkersStatus(ttl)
	lst, _ := s.Store.ListWorkers(service, ttl)
	writeJSON(w, lst, 200)
}

func (s *Server) handleAllocate(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	_ = s.Store.RefreshWorkersStatus(15)
	lst, _ := s.Store.ListWorkers(service, 15)
	if len(lst) == 0 {
		writeJSON(w, map[string]string{"error": "no worker"}, 500)
		return
	}
	best := lst[0]
	for _, wkr := range lst {
		if wkr.Load < best.Load {
			best = wkr
		}
	}
	writeJSON(w, best, 200)
}

type PaginatedResponse struct {
	Data  interface{} `json:"data"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
}

func (s *Server) handleFlows(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var payload struct {
			Name        string
			Description string
		}
		dec := json.NewDecoder(r.Body)
		_ = dec.Decode(&payload)
		id, err := s.Store.CreateFlow(payload.Name, payload.Description)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()}, 500)
			return
		}
		writeJSON(w, map[string]string{"id": id}, 200)
		return
	} else if r.Method == http.MethodGet {
		pageS := r.URL.Query().Get("page")
		pageSizeS := r.URL.Query().Get("page_size")
		page, _ := strconv.Atoi(pageS)
		if page < 1 {
			page = 1
		}
		pageSize, _ := strconv.Atoi(pageSizeS)
		if pageSize < 1 {
			pageSize = 10
		}
		offset := (page - 1) * pageSize

		flows, total, err := s.Store.ListFlows(pageSize, offset)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()}, 500)
			return
		}
		writeJSON(w, PaginatedResponse{
			Data:  flows,
			Total: total,
			Page:  page,
			Size:  pageSize,
		}, 200)
		return
	}
	writeJSON(w, map[string]string{"error": "method"}, 405)
}

func (s *Server) handleFlowVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var payload struct {
			FlowID         string
			Version        int
			DefinitionJSON string
			Status         string
		}
		dec := json.NewDecoder(r.Body)
		_ = dec.Decode(&payload)
		id, err := s.Store.CreateFlowVersion(payload.FlowID, payload.Version, payload.DefinitionJSON, payload.Status)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()}, 500)
			return
		}
		writeJSON(w, map[string]string{"id": id}, 200)
		return
	} else if r.Method == http.MethodGet {
		flowID := r.URL.Query().Get("flow_id")
		if flowID == "" {
			writeJSON(w, map[string]string{"error": "missing flow_id"}, 400)
			return
		}
		versions, err := s.Store.ListFlowVersions(flowID)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()}, 500)
			return
		}
		writeJSON(w, versions, 200)
		return
	}
	writeJSON(w, map[string]string{"error": "method"}, 405)
}

func (s *Server) handleGetFlowVersion(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, map[string]string{"error": "missing id"}, 400)
		return
	}
	fv, err := s.Store.GetFlowVersionByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, map[string]string{"error": "not found"}, 404)
			return
		}
		writeJSON(w, map[string]string{"error": err.Error()}, 500)
		return
	}
	writeJSON(w, fv, 200)
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var payload struct {
			FlowID     string
			Version    int
			ParamsJSON string
		}
		dec := json.NewDecoder(r.Body)
		_ = dec.Decode(&payload)
		var fv store.FlowVersion
		var err error
		if payload.Version == 0 {
			fv, err = s.Store.LatestPublishedVersion(payload.FlowID)
		} else {
			fv, err = s.Store.GetFlowVersionByFlowIDAndVersion(payload.FlowID, payload.Version)
		}
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()}, 500)
			return
		}
		var def struct{ Start string }
		_ = json.Unmarshal([]byte(fv.DefinitionJSON), &def)
		if def.Start == "" {
			writeJSON(w, map[string]string{"error": "no start"}, 400)
			return
		}
		id, err := s.Store.CreateTask(fv.ID, payload.ParamsJSON, "", def.Start)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()}, 500)
			return
		}
		writeJSON(w, map[string]string{"id": id}, 200)
		return
	} else if r.Method == http.MethodGet {
		status := r.URL.Query().Get("status")
		flowVersionID := r.URL.Query().Get("flow_version_id")
		pageS := r.URL.Query().Get("page")
		pageSizeS := r.URL.Query().Get("page_size")
		page, _ := strconv.Atoi(pageS)
		if page < 1 {
			page = 1
		}
		pageSize, _ := strconv.Atoi(pageSizeS)
		if pageSize < 1 {
			pageSize = 10
		}
		offset := (page - 1) * pageSize

		tasks, total, err := s.Store.ListTasks(status, flowVersionID, pageSize, offset)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()}, 500)
			return
		}
		writeJSON(w, PaginatedResponse{
			Data:  tasks,
			Total: total,
			Page:  page,
			Size:  pageSize,
		}, 200)
		return
	}
	writeJSON(w, map[string]string{"error": "method"}, 405)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	t, err := s.Store.GetTask(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, map[string]string{"error": "not found"}, 404)
			return
		}
		writeJSON(w, map[string]string{"error": err.Error()}, 500)
		return
	}
	writeJSON(w, t, 200)
}

func (s *Server) handleRunOnce(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, map[string]string{"error": "method"}, 405)
		return
	}
	id := r.URL.Query().Get("id")
	eng := engine.New(s.Store)
	owner := r.URL.Query().Get("owner")
	if owner == "" {
		owner = "manual"
	}
	eng.Owner = owner
	err := eng.RunOnce(id)
	if err != nil {
		code := 500
		if err.Error() == "lease_mismatch" || err.Error() == "lease_expired" {
			code = 409
		}
		writeJSON(w, map[string]string{"error": err.Error()}, code)
		return
	}
	writeJSON(w, map[string]string{"ok": "1"}, 200)
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, map[string]string{"error": "method"}, 405)
		return
	}
	id := r.URL.Query().Get("id")
	_ = s.Store.UpdateTaskStatus(id, "canceling")
	writeJSON(w, map[string]string{"ok": "1"}, 200)
}

func (s *Server) handleTaskRuns(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("task_id")
	runs, err := s.Store.ListNodeRuns(id)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()}, 500)
		return
	}
	writeJSON(w, runs, 200)
}

func (s *Server) handleTaskSignal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, map[string]string{"error": "method"}, 405)
		return
	}
	var payload struct {
		TaskID string      `json:"task_id"`
		Key    string      `json:"key"`
		Value  interface{} `json:"value"`
	}
	dec := json.NewDecoder(r.Body)
	_ = dec.Decode(&payload)
	if payload.TaskID == "" || payload.Key == "" {
		writeJSON(w, map[string]string{"error": "bad request"}, 400)
		return
	}
	t, err := s.Store.GetTask(payload.TaskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, map[string]string{"error": "not found"}, 404)
			return
		}
		writeJSON(w, map[string]string{"error": err.Error()}, 500)
		return
	}
	var shared map[string]interface{}
	_ = json.Unmarshal([]byte(t.SharedJSON), &shared)
	if shared == nil {
		shared = map[string]interface{}{}
	}
	shared[payload.Key] = payload.Value
	sb, _ := json.Marshal(shared)
	_ = s.Store.UpdateTaskProgress(payload.TaskID, t.CurrentNodeKey, "", string(sb), t.StepCount)
	writeJSON(w, map[string]string{"ok": "1"}, 200)
}
