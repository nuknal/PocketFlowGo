package engine

import (
	"encoding/json"

	"github.com/nuknal/PocketFlowGo/internal/store"
)

// execQueue handles execution via the persistent task queue (Pull Mode).
func (e *Engine) execQueue(t store.Task, node DefNode, curr string, input interface{}, params map[string]interface{}) (interface{}, string, string, error) {
	// 1. Check if we already have a completed run for this node
	// If the task was in "waiting_queue" and we are here, it means the scheduler picked it up.
	// We need to check if there is a successful node_run for this node_key that happened AFTER the task was last updated (or just the latest one).

	runs, err := e.Store.ListNodeRuns(t.ID)
	if err == nil && len(runs) > 0 {
		// Look for the latest run for this node
		var lastRun *store.NodeRun
		for i := len(runs) - 1; i >= 0; i-- {
			if runs[i].NodeKey == curr {
				lastRun = &runs[i]
				break
			}
		}

		if lastRun != nil && lastRun.Status == "ok" {
			// Found a completed run! Return the result.
			var res interface{}
			if err := json.Unmarshal([]byte(lastRun.ExecOutputJSON), &res); err != nil {
				return nil, "queue", "queue", errorString("failed to parse result")
			}
			return res, lastRun.WorkerID, "queue", nil
		}

		if lastRun != nil && lastRun.Status == "error" {
			return nil, lastRun.WorkerID, "queue", errorString(lastRun.ErrorText)
		}
	}

	// 2. If no result, enqueue the task
	payload := map[string]interface{}{"input": input, "params": params}
	inputJSON, _ := json.Marshal(payload)

	_, err = e.Store.EnqueueTask(t.ID, curr, node.Service, string(inputJSON))
	if err != nil {
		return nil, "", "", err
	}

	// 3. Return special error to suspend execution
	return nil, "queue", "queue", ErrAsyncPending
}
