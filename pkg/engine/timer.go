package engine

import (
	"time"
)

// runTimer executes a 'timer' node, which pauses execution for a specified duration.
func (e *Engine) runTimer(in NodeRunInput) error {
	// Initialize runtime state for timer
	rt, _ := in.Shared["_rt"].(map[string]interface{})
	if rt == nil {
		rt = map[string]interface{}{}
	}
	key := "tm:" + in.NodeKey
	tm, _ := rt[key].(map[string]interface{})
	now := time.Now().UnixMilli()

	// Start timer if not already running
	if tm == nil {
		tm = map[string]interface{}{"start": now}
		rt[key] = tm
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

	// Calculate delay
	delay := 0
	if v, ok := in.Params["delay_ms"].(float64); ok {
		delay = int(v)
	} else if v2, ok := in.Params["delay_ms"].(int); ok {
		delay = v2
	}
	start := int64(0)
	if s, ok := tm["start"].(int64); ok {
		start = s
	}
	if start == 0 {
		if s2, ok := tm["start"].(float64); ok {
			start = int64(s2)
		}
	}

	// Check if timer has expired
	if delay <= 0 || now-start >= int64(delay) {
		action := in.Node.Post.ActionStatic
		if in.Node.Post.OutputKey != "" {
			in.Shared[in.Node.Post.OutputKey] = in.Input
		}
		delete(rt, key)
		if len(rt) == 0 {
			delete(in.Shared, "_rt")
		} else {
			in.Shared["_rt"] = rt
		}
		e.recordRun(in.Task, in.NodeKey, 1, "ok", map[string]interface{}{"delay_ms": delay}, in.Input, nil, "", action, "", "", "")
		return e.finishNode(in.Task, in.FlowDef, in.NodeKey, action, in.Shared, in.Task.StepCount+1, nil)
	}

	// If not expired, update status and continue waiting
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
