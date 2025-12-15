package engine

import (
	"time"
)

// runWaitEvent executes a 'wait_event' node, pausing execution until a signal is received or timeout occurs.
func (e *Engine) runWaitEvent(in NodeRunInput) error {
	// Initialize runtime state
	rt, _ := in.Shared["_rt"].(map[string]interface{})
	if rt == nil {
		rt = map[string]interface{}{}
	}
	key := "we:" + in.NodeKey
	we, _ := rt[key].(map[string]interface{})
	if we == nil {
		we = map[string]interface{}{"start": time.Now().UnixMilli()}
	}

	// Resolve signal key and timeout configuration
	signalKey := ""
	if v, ok := in.Params["signal_key"].(string); ok {
		signalKey = v
	}
	sig := resolveRef(signalKey, in.Shared, in.Params, in.Input)
	timeout := 0
	if v, ok := in.Params["timeout_ms"].(float64); ok {
		timeout = int(v)
	} else if v2, ok := in.Params["timeout_ms"].(int); ok {
		timeout = v2
	}
	strat := in.Node.FailureStrategy

	// Check if signal received
	if sig != nil && sig != "" && sig != false {
		action := in.Node.Post.ActionStatic
		if action == "" && in.Node.Post.ActionKey != "" {
			action = pickAction(map[string]interface{}{"signal": sig}, in.Node.Post.ActionKey)
		}
		if in.Node.Post.OutputKey != "" {
			in.Shared[in.Node.Post.OutputKey] = sig
		}
		delete(rt, key)
		if len(rt) == 0 {
			delete(in.Shared, "_rt")
		} else {
			in.Shared["_rt"] = rt
		}
		e.recordRun(in.Task, in.NodeKey, 1, "ok", map[string]interface{}{"signal_key": signalKey}, in.Input, sig, "", action, "", "", "")
		return e.finishNode(in.Task, in.FlowDef, in.NodeKey, action, in.Shared, in.Task.StepCount+1, nil)
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
			in.Shared["_rt"] = rt
			if e.Owner != "" {
				_ = e.Store.UpdateTaskStatusOwned(in.Task.ID, e.Owner, "running")
			} else {
				_ = e.Store.UpdateTaskStatus(in.Task.ID, "running")
			}
			if e.Owner != "" {
				_ = e.Store.UpdateTaskProgressOwned(in.Task.ID, e.Owner, in.NodeKey, "", toJSON(in.Shared), in.Task.StepCount+1)
			} else {
				_ = e.Store.UpdateTaskProgress(in.Task.ID, in.NodeKey, "", toJSON(in.Shared), in.Task.StepCount+1)
			}
			return nil
		}
		action := in.Node.Post.ActionStatic
		if strat == "continue" {
			delete(rt, key)
			if len(rt) == 0 {
				delete(in.Shared, "_rt")
			} else {
				in.Shared["_rt"] = rt
			}
			e.recordRun(in.Task, in.NodeKey, 1, "ok", map[string]interface{}{"signal_key": signalKey}, in.Input, nil, "", action, "", "", "")
			return e.finishNode(in.Task, in.FlowDef, in.NodeKey, action, in.Shared, in.Task.StepCount+1, nil)
		}
		// Default timeout behavior: fail
		delete(rt, key)
		if len(rt) == 0 {
			delete(in.Shared, "_rt")
		} else {
			in.Shared["_rt"] = rt
		}
		e.recordRun(in.Task, in.NodeKey, 1, "error", map[string]interface{}{"signal_key": signalKey}, in.Input, nil, "timeout", action, "", "", "")
		return e.finishNode(in.Task, in.FlowDef, in.NodeKey, action, in.Shared, in.Task.StepCount+1, errorString("timeout"))
	}

	// Update state and wait
	rt[key] = we
	in.Shared["_rt"] = rt
	if e.Owner != "" {
		_ = e.Store.UpdateTaskStatusOwned(in.Task.ID, e.Owner, "running")
	} else {
		_ = e.Store.UpdateTaskStatus(in.Task.ID, "running")
	}
	if e.Owner != "" {
		_ = e.Store.UpdateTaskProgressOwned(in.Task.ID, e.Owner, in.NodeKey, "", toJSON(in.Shared), in.Task.StepCount+1)
	} else {
		_ = e.Store.UpdateTaskProgress(in.Task.ID, in.NodeKey, "", toJSON(in.Shared), in.Task.StepCount+1)
	}
	return nil
}
