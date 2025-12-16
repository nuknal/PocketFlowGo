package engine

import (
	"encoding/json"
	"time"

	"github.com/nuknal/PocketFlowGo/internal/store"
)

// execQueue handles execution via the persistent task queue (Pull Mode).
func (e *Engine) execQueue(in ExecutorInput) ExecutorResult {
	// 1. Check if we already have a completed run for this node
	// If the task was in "waiting_queue" and we are here, it means the scheduler picked it up.
	// We need to check if there is a successful node_run for this node_key that happened AFTER the task was last updated (or just the latest one).

	runs, err := e.Store.ListNodeRuns(in.Task.ID)
	if err == nil && len(runs) > 0 {
		// Look for the latest run for this node
		var lastRun *store.NodeRun
		for i := len(runs) - 1; i >= 0; i-- {
			if runs[i].NodeKey == in.NodeKey {
				lastRun = &runs[i]
				break
			}
		}

		if lastRun != nil {
			if lastRun.Status == "ok" {
				// Found a completed run! Return the result.
				var res interface{}
				if err := json.Unmarshal([]byte(lastRun.ExecOutputJSON), &res); err != nil {
					return ExecutorResult{WorkerID: "queue", WorkerURL: "queue", Error: errorString("failed to parse result")}
				}
				return ExecutorResult{Result: res, WorkerID: lastRun.WorkerID, WorkerURL: "queue", LogPath: lastRun.LogPath, SkipRecord: true}
			}

			if lastRun.Status == "error" {
				return ExecutorResult{WorkerID: lastRun.WorkerID, WorkerURL: "queue", LogPath: lastRun.LogPath, Error: errorString(lastRun.ErrorText), SkipRecord: true}
			}

			// If already running or queued, don't re-enqueue
			if lastRun.Status == "queued" || lastRun.Status == "running" {
				return ExecutorResult{WorkerID: "queue", WorkerURL: "queue", Error: ErrAsyncPending}
			}
		}
	}

	// 2. If no result, enqueue the task
	// Create a new node_run with status "queued"
	runID := store.GenID("run")

	// Create the node_run record
	nr := map[string]interface{}{
		"id":               runID,
		"task_id":          in.Task.ID,
		"node_key":         in.NodeKey,
		"attempt_no":       1,
		"status":           "queued",
		"prep_json":        toJSON(map[string]interface{}{"input_key": in.Node.Prep.InputKey}),
		"exec_input_json":  toJSON(in.Input),
		"exec_output_json": toJSON(nil),
		"error_text":       "",
		"action":           "",
		"started_at":       time.Now().Unix(),
		"finished_at":      0, // Not finished
		"worker_id":        "queue",
		"worker_url":       "queue",
		"log_path":         "",
	}
	if err := e.Store.CreateNodeRun(nr); err != nil {
		e.logf("failed to create queued node_run: %v", err)
	}

	payload := map[string]interface{}{
		"input":  in.Input,
		"params": in.Params,
		"run_id": runID,
	}
	inputJSON, _ := json.Marshal(payload)

	_, err = e.Store.EnqueueTask(in.Task.ID, in.NodeKey, in.Node.Service, string(inputJSON))
	if err != nil {
		return ExecutorResult{Error: err}
	}

	// 3. Return special error to suspend execution
	return ExecutorResult{WorkerID: "queue", WorkerURL: "queue", Error: ErrAsyncPending}
}
