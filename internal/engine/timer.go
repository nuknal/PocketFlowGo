package engine

import (
	"time"

	"github.com/nuknal/PocketFlowGo/internal/store"
)

// runTimer executes a 'timer' node, which pauses execution for a specified duration.
func (e *Engine) runTimer(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, params map[string]interface{}, input interface{}) error {
	// Initialize runtime state for timer
	rt, _ := shared["_rt"].(map[string]interface{})
	if rt == nil {
		rt = map[string]interface{}{}
	}
	key := "tm:" + curr
	tm, _ := rt[key].(map[string]interface{})
	now := time.Now().UnixMilli()
	
	// Start timer if not already running
	if tm == nil {
		tm = map[string]interface{}{"start": now}
		rt[key] = tm
		shared["_rt"] = rt
        if e.Owner != "" { _ = e.Store.UpdateTaskStatusOwned(t.ID, e.Owner, "running") } else { _ = e.Store.UpdateTaskStatus(t.ID, "running") }
        if e.Owner != "" { _ = e.Store.UpdateTaskProgressOwned(t.ID, e.Owner, curr, "", toJSON(shared), t.StepCount+1) } else { _ = e.Store.UpdateTaskProgress(t.ID, curr, "", toJSON(shared), t.StepCount+1) }
		return nil
	}

	// Calculate delay
	delay := 0
	if v, ok := params["delay_ms"].(float64); ok {
		delay = int(v)
	} else if v2, ok := params["delay_ms"].(int); ok {
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
		action := node.Post.ActionStatic
		if node.Post.OutputKey != "" {
			shared[node.Post.OutputKey] = input
		}
		delete(rt, key)
		if len(rt) == 0 {
			delete(shared, "_rt")
		} else {
			shared["_rt"] = rt
		}
		e.recordRun(t, curr, 1, "ok", map[string]interface{}{"delay_ms": delay}, input, nil, "", action, "", "")
		return e.finishNode(t, def, curr, action, shared, t.StepCount+1, nil)
	}
	
	// If not expired, update status and continue waiting
    if e.Owner != "" { _ = e.Store.UpdateTaskStatusOwned(t.ID, e.Owner, "running") } else { _ = e.Store.UpdateTaskStatus(t.ID, "running") }
    if e.Owner != "" { _ = e.Store.UpdateTaskProgressOwned(t.ID, e.Owner, curr, "", toJSON(shared), t.StepCount+1) } else { _ = e.Store.UpdateTaskProgress(t.ID, curr, "", toJSON(shared), t.StepCount+1) }
	return nil
}
