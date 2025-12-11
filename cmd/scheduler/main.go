package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/nuknal/PocketFlowGo/internal/api"
	"github.com/nuknal/PocketFlowGo/internal/engine"
	"github.com/nuknal/PocketFlowGo/internal/store"
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
				_ = eng.RunOnce(t.ID)
				nt, _ := s.GetTask(t.ID)
				if nt.Status == "completed" || nt.CurrentNodeKey == "" {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
	http.ListenAndServe(":8070", mux)
}
