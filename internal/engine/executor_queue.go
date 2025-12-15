package engine

import (
	"encoding/json"

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

		if lastRun != nil && lastRun.Status == "ok" {
			// Found a completed run! Return the result.
			var res interface{}
			if err := json.Unmarshal([]byte(lastRun.ExecOutputJSON), &res); err != nil {
				return ExecutorResult{WorkerID: "queue", WorkerURL: "queue", Error: errorString("failed to parse result")}
			}
			return ExecutorResult{Result: res, WorkerID: lastRun.WorkerID, WorkerURL: "queue", LogPath: lastRun.LogPath}
		}

		if lastRun != nil && lastRun.Status == "error" {
			return ExecutorResult{WorkerID: lastRun.WorkerID, WorkerURL: "queue", LogPath: lastRun.LogPath, Error: errorString(lastRun.ErrorText)}
		}
	}

	// 2. If no result, enqueue the task
	payload := map[string]interface{}{"input": in.Input, "params": in.Params}
	inputJSON, _ := json.Marshal(payload)

	_, err = e.Store.EnqueueTask(in.Task.ID, in.NodeKey, in.Node.Service, string(inputJSON))
	if err != nil {
		return ExecutorResult{Error: err}
	}

	// 3. Return special error to suspend execution
	return ExecutorResult{WorkerID: "queue", WorkerURL: "queue", Error: ErrAsyncPending}
}
