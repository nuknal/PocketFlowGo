package main

import (
	"net/http"
	"os"
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
		eng := engine.New(s)
		owner := "scheduler"
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
