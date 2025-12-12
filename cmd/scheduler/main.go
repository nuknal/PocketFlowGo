package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nuknal/PocketFlowGo/internal/api"
	"github.com/nuknal/PocketFlowGo/internal/engine"
	"github.com/nuknal/PocketFlowGo/internal/store"
	"github.com/nuknal/PocketFlowGo/ui"
)

func main() {
	dbpath := os.Getenv("SCHEDULER_DB")
	if dbpath == "" {
		dbpath = "scheduler.db"
	}
	s, err := store.OpenSQLite(dbpath)
	if err != nil {
		panic(err)
	}
	srv := &api.Server{Store: s}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// Serve UI
	distFS, err := fs.Sub(ui.Assets, "dist")
	if err != nil {
		panic(err)
	}
	// SPA fallback handler
	fileServer := http.FileServer(http.FS(distFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// If path is API, let it 404 naturally if not handled
		if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
			http.NotFound(w, r)
			return
		}

		// Check if file exists in FS
		path := r.URL.Path
		if path == "/" {
			path = "index.html"
		} else if path[0] == '/' {
			path = path[1:]
		}

		f, err := distFS.Open(path)
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

	go func() {
		ttl := int64(15)
		if v := os.Getenv("WORKER_OFFLINE_TTL_SEC"); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				ttl = n
			}
		}
		interval := int64(5)
		if v := os.Getenv("WORKER_REFRESH_INTERVAL_SEC"); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				interval = n
			}
		}
		for {
			_ = s.RefreshWorkersStatus(ttl)
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}()
	go func() {
		eng := engine.New(s)
		eng.RegisterFunc("mul", func(ctx context.Context, input interface{}, params map[string]interface{}) (interface{}, error) {
			f := 0.0
			if v, ok := input.(float64); ok {
				f = v
			}
			m := 1.0
			if mv, ok := params["mul"].(float64); ok {
				m = mv
			}
			return f * m, nil
		})
		eng.RegisterFunc("upper", func(ctx context.Context, input interface{}, params map[string]interface{}) (interface{}, error) {
			fmt.Printf("UPPER: %v\n", input)
			if s, ok := input.(string); ok {
				return strings.ToUpper(s), nil
			}
			if s, ok := params["text"].(string); ok {
				return strings.ToUpper(s), nil
			}
			return nil, fmt.Errorf("expected string input")
		})
		eng.RegisterFunc("log_result", func(ctx context.Context, input interface{}, params map[string]interface{}) (interface{}, error) {
			fmt.Printf("LOG RESULT: %v\n", input)
			return input, nil
		})
		owner := os.Getenv("SCHEDULER_OWNER")
		if owner == "" {
			hn, _ := os.Hostname()
			owner = fmt.Sprintf("scheduler-%s-%d-%d", hn, os.Getpid(), time.Now().UnixNano())
		}
		eng.Owner = owner
		ttl := int64(3)
		for {
			t, err := s.LeaseNextTask(owner, ttl)
			if err != nil {
				time.Sleep(300 * time.Millisecond)
				continue
			}
			for {
				_ = s.ExtendLease(t.ID, owner, ttl)
				if err := eng.RunOnce(t.ID); err != nil {
					// Check if it's a fatal/system error or just a regular execution failure.
					// RunOnce normally handles node failures by updating status to failed (via finishNode).
					// But if it returns an error here, it means something prevented it from finishing the node (e.g. panic, db error).
					// We must mark it as failed to stop the loop.
					// Note: ErrAsyncPending is handled inside RunOnce (suspends task), so RunOnce returns nil for it?
					// Let's check executor.go: "return e.suspendTask(...)". suspendTask returns error.
					// So if suspended, RunOnce returns error?
					// suspendTask returns error only if DB update fails.
					// So normally RunOnce returns nil even if suspended.
					log.Printf("RunOnce error for task %s: %v", t.ID, err)
					_ = s.UpdateTaskStatus(t.ID, "failed")
					break
				}
				nt, _ := s.GetTask(t.ID)
				if nt.Status == "completed" || nt.Status == "failed" || nt.Status == "waiting_queue" || nt.CurrentNodeKey == "" {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
	http.ListenAndServe(":8070", mux)
}
