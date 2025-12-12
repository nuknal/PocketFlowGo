package engine

import (
	"time"

	"github.com/nuknal/PocketFlowGo/internal/store"
)

// runExecutorNode executes a node of kind 'executor'.
// It handles retries, input/output mapping, and transitions.
func (e *Engine) runExecutorNode(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, params map[string]interface{}, input interface{}) error {
	var execRes interface{}
	var workerID, workerURL string
	var execErr error
	action := ""
	attempts := 0

	// Loop for retries
	for {
		attempts++
		// Execute the logic
		execRes, workerID, workerURL, execErr = e.execExecutor(t, node, curr, input, params)

		// Handle Async Queue suspension
		if execErr == ErrAsyncPending {
			return e.suspendTask(t, "waiting_queue", shared)
		}

		// Log and record execution attempt
		e.logf("task=%s node=%s kind=executor attempt=%d worker=%s status=%s", t.ID, curr, attempts, workerID, ternary(execErr == nil, "ok", "error"))
		e.recordRun(t, curr, attempts, ternary(execErr == nil, "ok", "error"), map[string]interface{}{"input_key": node.Prep.InputKey}, input, execRes, errString(execErr), action, workerID, workerURL)

		if execErr == nil {
			break
		}

		// Check retry limits
		if execErr == ErrFatal || attempts > node.MaxRetries {
			break
		}

		// Wait before retry
		if node.WaitMillis > 0 {
			time.Sleep(time.Duration(node.WaitMillis) * time.Millisecond)
		}
	}

	// If execution succeeded, handle outputs and determine next action
	if execErr == nil {
		if node.Post.OutputMap != nil {
			if mm, ok := execRes.(map[string]interface{}); ok {
				for toKey, fromField := range node.Post.OutputMap {
					shared[toKey] = mm[fromField]
				}
			}
		}
		if node.Post.OutputKey != "" {
			shared[node.Post.OutputKey] = execRes
		}

		// Determine transition
		if node.Post.ActionStatic != "" {
			action = node.Post.ActionStatic
		} else if node.Post.ActionKey != "" {
			action = pickAction(execRes, node.Post.ActionKey)
		}
	}
	return e.finishNode(t, def, curr, action, shared, t.StepCount+1, execErr)
}

// execExecutor dispatches execution to the appropriate handler based on ExecType.
func (e *Engine) execExecutor(t store.Task, node DefNode, curr string, input interface{}, params map[string]interface{}) (interface{}, string, string, error) {
	et := node.ExecType
	if et == "" {
		et = "http"
	}
	switch et {
	case "http":
		return e.execHTTP(node, input, params)
	case "local_func":
		return e.execLocalFunc(node, input, params)
	case "local_script":
		return e.execLocalScript(node, input, params)
	case "queue":
		return e.execQueue(t, node, curr, input, params)
	default:
		return nil, "", "", errorString("unsupported exec")
	}
}
