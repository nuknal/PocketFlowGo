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

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/workers/register", withCORS(s.handleRegisterWorker))
	mux.HandleFunc("/workers/heartbeat", withCORS(s.handleHeartbeat))
	mux.HandleFunc("/workers/list", withCORS(s.handleListWorkers))
	mux.HandleFunc("/workers/allocate", withCORS(s.handleAllocate))
	mux.HandleFunc("/register", withCORS(s.handleRegisterWorker))
	mux.HandleFunc("/heartbeat", withCORS(s.handleHeartbeat))
	mux.HandleFunc("/list", withCORS(s.handleListWorkers))
	mux.HandleFunc("/allocate", withCORS(s.handleAllocate))
	mux.HandleFunc("/flows", withCORS(s.handleFlows))
	mux.HandleFunc("/flows/version", withCORS(s.handleFlowVersion))
	mux.HandleFunc("/tasks", withCORS(s.handleTasks))
	mux.HandleFunc("/tasks/get", withCORS(s.handleGetTask))
	mux.HandleFunc("/tasks/run_once", withCORS(s.handleRunOnce))
	mux.HandleFunc("/tasks/cancel", withCORS(s.handleCancel))
	mux.HandleFunc("/tasks/runs", withCORS(s.handleTaskRuns))
	mux.HandleFunc("/tasks/signal", withCORS(s.handleTaskSignal))
}

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

func (s *Server) handleFlows(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var payload struct{ Name string }
		dec := json.NewDecoder(r.Body)
		_ = dec.Decode(&payload)
		id, err := s.Store.CreateFlow(payload.Name)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()}, 500)
			return
		}
		writeJSON(w, map[string]string{"id": id}, 200)
		return
	} else if r.Method == http.MethodGet {
		flows, err := s.Store.ListFlows()
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()}, 500)
			return
		}
		writeJSON(w, flows, 200)
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
			fv, err = s.Store.LatestPublishedVersion(payload.FlowID)
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
		tasks, err := s.Store.ListTasks(status, 100)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()}, 500)
			return
		}
		writeJSON(w, tasks, 200)
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
