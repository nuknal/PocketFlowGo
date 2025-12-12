package engine

import (
	"time"

	"github.com/nuknal/PocketFlowGo/internal/store"
)

// runWaitEvent executes a 'wait_event' node, pausing execution until a signal is received or timeout occurs.
func (e *Engine) runWaitEvent(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, params map[string]interface{}, input interface{}) error {
	// Initialize runtime state
	rt, _ := shared["_rt"].(map[string]interface{})
	if rt == nil {
		rt = map[string]interface{}{}
	}
	key := "we:" + curr
	we, _ := rt[key].(map[string]interface{})
	if we == nil {
		we = map[string]interface{}{"start": time.Now().UnixMilli()}
	}

	// Resolve signal key and timeout configuration
	signalKey := ""
	if v, ok := params["signal_key"].(string); ok {
		signalKey = v
	}
	sig := resolveRef(signalKey, shared, params, input)
	timeout := 0
	if v, ok := params["timeout_ms"].(float64); ok {
		timeout = int(v)
	} else if v2, ok := params["timeout_ms"].(int); ok {
		timeout = v2
	}
	strat := node.FailureStrategy

	// Check if signal received
	if sig != nil && sig != "" && sig != false {
		action := node.Post.ActionStatic
		if action == "" && node.Post.ActionKey != "" {
			action = pickAction(map[string]interface{}{"signal": sig}, node.Post.ActionKey)
		}
		if node.Post.OutputKey != "" {
			shared[node.Post.OutputKey] = sig
		}
		delete(rt, key)
		if len(rt) == 0 {
			delete(shared, "_rt")
		} else {
			shared["_rt"] = rt
		}
		e.recordRun(t, curr, 1, "ok", map[string]interface{}{"signal_key": signalKey}, input, sig, "", action, "", "")
		return e.finishNode(t, def, curr, action, shared, t.StepCount+1, nil)
	}

	// Check for timeout
	start := int64(0)
	if s, ok := we["start"].(int64); ok {
		start = s
	} else if s2, ok := we["start"].(float64); ok {
		start = int64(s2)
	}
	if timeout > 0 && time.Now().UnixMilli()-start >= int64(timeout) {
		// Handle timeout strategies
		if strat == "retry" {
			we["start"] = time.Now().UnixMilli()
			rt[key] = we
			shared["_rt"] = rt
            if e.Owner != "" { _ = e.Store.UpdateTaskStatusOwned(t.ID, e.Owner, "running") } else { _ = e.Store.UpdateTaskStatus(t.ID, "running") }
            if e.Owner != "" { _ = e.Store.UpdateTaskProgressOwned(t.ID, e.Owner, curr, "", toJSON(shared), t.StepCount+1) } else { _ = e.Store.UpdateTaskProgress(t.ID, curr, "", toJSON(shared), t.StepCount+1) }
			return nil
		}
		action := node.Post.ActionStatic
		if strat == "continue" {
			delete(rt, key)
			if len(rt) == 0 {
				delete(shared, "_rt")
			} else {
				shared["_rt"] = rt
			}
			e.recordRun(t, curr, 1, "ok", map[string]interface{}{"signal_key": signalKey}, input, nil, "", action, "", "")
			return e.finishNode(t, def, curr, action, shared, t.StepCount+1, nil)
		}
		// Default timeout behavior: fail
		delete(rt, key)
		if len(rt) == 0 {
			delete(shared, "_rt")
		} else {
			shared["_rt"] = rt
		}
		e.recordRun(t, curr, 1, "error", map[string]interface{}{"signal_key": signalKey}, input, nil, "timeout", action, "", "")
		return e.finishNode(t, def, curr, action, shared, t.StepCount+1, errorString("timeout"))
	}

	// Update state and wait
	rt[key] = we
	shared["_rt"] = rt
    if e.Owner != "" { _ = e.Store.UpdateTaskStatusOwned(t.ID, e.Owner, "running") } else { _ = e.Store.UpdateTaskStatus(t.ID, "running") }
    if e.Owner != "" { _ = e.Store.UpdateTaskProgressOwned(t.ID, e.Owner, curr, "", toJSON(shared), t.StepCount+1) } else { _ = e.Store.UpdateTaskProgress(t.ID, curr, "", toJSON(shared), t.StepCount+1) }
	return nil
}
