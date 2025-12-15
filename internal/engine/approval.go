package engine

func (e *Engine) runApproval(in NodeRunInput) error {
	// Initialize runtime state for approval if not exists
	rt, _ := in.Shared["_rt"].(map[string]interface{})
	if rt == nil {
		rt = map[string]interface{}{}
	}
	key := "ap:" + in.NodeKey
	ap, _ := rt[key].(map[string]interface{})
	if ap == nil {
		ap = map[string]interface{}{}
	}

	// Resolve the approval value from params
	approvalKey := ""
	if v, ok := in.Params["approval_key"].(string); ok {
		approvalKey = v
	}
	val := resolveRef(approvalKey, in.Shared, in.Params, in.Input)

	decided := false
	action := in.Node.Post.ActionStatic

	// Check if approval decision has been made
	if val != nil && val != "" {
		decided = true
		if in.Node.Post.ActionKey != "" {
			action = pickAction(map[string]interface{}{"approval": val}, in.Node.Post.ActionKey)
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
		if in.Node.Post.OutputKey != "" {
			in.Shared[in.Node.Post.OutputKey] = val
		}
		// Cleanup runtime state
		delete(rt, key)
		if len(rt) == 0 {
			delete(in.Shared, "_rt")
		} else {
			in.Shared["_rt"] = rt
		}
		e.recordRun(in.Task, in.NodeKey, 1, "ok", map[string]interface{}{"approval_key": approvalKey}, in.Input, val, "", action, "", "", "")
		return e.finishNode(in.Task, in.FlowDef, in.NodeKey, action, in.Shared, in.Task.StepCount+1, nil)
	}

	// If not decided, suspend execution and wait
	rt[key] = ap
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
