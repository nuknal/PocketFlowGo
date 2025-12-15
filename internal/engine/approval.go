package engine

import (
	"github.com/nuknal/PocketFlowGo/internal/store"
)

func (e *Engine) runApproval(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, params map[string]interface{}, input interface{}) error {
	// Initialize runtime state for approval if not exists
	rt, _ := shared["_rt"].(map[string]interface{})
	if rt == nil {
		rt = map[string]interface{}{}
	}
	key := "ap:" + curr
	ap, _ := rt[key].(map[string]interface{})
	if ap == nil {
		ap = map[string]interface{}{}
	}

	// Resolve the approval value from params
	approvalKey := ""
	if v, ok := params["approval_key"].(string); ok {
		approvalKey = v
	}
	val := resolveRef(approvalKey, shared, params, input)

	decided := false
	action := node.Post.ActionStatic

	// Check if approval decision has been made
	if val != nil && val != "" {
		decided = true
		if node.Post.ActionKey != "" {
			action = pickAction(map[string]interface{}{"approval": val}, node.Post.ActionKey)
		} else {
			switch vv := val.(type) {
			case bool:
				if vv {
					action = "approved"
				} else {
					action = "rejected"
				}
			case string:
				if vv != "" {
					action = vv
				}
			}
		}
	}

	// If decided, proceed to next step
	if decided {
		if node.Post.OutputKey != "" {
			shared[node.Post.OutputKey] = val
		}
		// Cleanup runtime state
		delete(rt, key)
		if len(rt) == 0 {
			delete(shared, "_rt")
		} else {
			shared["_rt"] = rt
		}
		e.recordRun(t, curr, 1, "ok", map[string]interface{}{"approval_key": approvalKey}, input, val, "", action, "", "", "")
		return e.finishNode(t, def, curr, action, shared, t.StepCount+1, nil)
	}

	// If not decided, suspend execution and wait
	rt[key] = ap
	shared["_rt"] = rt
	if e.Owner != "" {
		_ = e.Store.UpdateTaskStatusOwned(t.ID, e.Owner, "running")
	} else {
		_ = e.Store.UpdateTaskStatus(t.ID, "running")
	}
	if e.Owner != "" {
		_ = e.Store.UpdateTaskProgressOwned(t.ID, e.Owner, curr, "", toJSON(shared), t.StepCount+1)
	} else {
		_ = e.Store.UpdateTaskProgress(t.ID, curr, "", toJSON(shared), t.StepCount+1)
	}
	return nil
}
