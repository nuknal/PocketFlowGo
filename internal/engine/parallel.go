package engine

import (
	"github.com/nuknal/PocketFlowGo/internal/store"
)

// runParallel executes multiple services in parallel (concurrently or sequentially).
func (e *Engine) runParallel(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, params map[string]interface{}, input interface{}) error {
	svcs := node.ParallelServices
	specs := map[string]ExecSpec{}

	// Collect configured services/executions
	if len(node.ParallelExecs) > 0 {
		svcs = []string{}
		for _, sp := range node.ParallelExecs {
			svcs = append(svcs, sp.Service)
			specs[sp.Service] = sp
		}
	}
	if len(svcs) == 0 {
		if arr, ok := params["services"].([]interface{}); ok {
			for _, x := range arr {
				if s, ok := x.(string); ok {
					svcs = append(svcs, s)
				}
			}
		}
	}

	// Handle no services case
	if len(svcs) == 0 {
		e.recordRun(t, curr, 1, "error", map[string]interface{}{"input_key": node.Prep.InputKey}, input, nil, "no services", "", "", "")
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

	// Initialize runtime state for parallel execution
	rt, _ := shared["_rt"].(map[string]interface{})
	if rt == nil {
		rt = map[string]interface{}{}
	}
	key := "pl:" + curr
	pl, _ := rt[key].(map[string]interface{})
	if pl == nil {
		pl = map[string]interface{}{"done": map[string]interface{}{}, "errs": map[string]interface{}{}, "mode": node.ParallelMode, "max": node.MaxParallel, "strategy": node.FailureStrategy}
	}
	done := pl["done"].(map[string]interface{})
	errs := pl["errs"].(map[string]interface{})

	// Determine remaining services
	remaining := []string{}
	for _, sname := range svcs {
		if _, ok := done[sname]; !ok {
			remaining = append(remaining, sname)
		}
	}
	e.logf("task=%s node=%s kind=parallel mode=%s remaining=%d total=%d", t.ID, curr, pl["mode"], len(remaining), len(svcs))

	// If all completed, aggregate results and finish
	if len(remaining) == 0 {
		agg := make([]interface{}, 0, len(svcs))
		for _, sname := range svcs {
			agg = append(agg, done[sname])
		}
		action := ""
		if node.Post.OutputKey != "" {
			shared[node.Post.OutputKey] = agg
		}
		if node.Post.ActionStatic != "" {
			action = node.Post.ActionStatic
		} else if node.Post.ActionKey != "" {
			action = pickAction(map[string]interface{}{"result": agg}, node.Post.ActionKey)
		}
		delete(rt, key)
		if len(rt) == 0 {
			delete(shared, "_rt")
		} else {
			shared["_rt"] = rt
		}
		hasErr := len(errs) != 0
		cont := node.FailureStrategy == "continue"
		e.recordRun(t, curr, 1, ternary(!hasErr || cont, "ok", "error"), map[string]interface{}{"input_key": node.Prep.InputKey}, input, agg, ternary(!hasErr || cont, "", toJSON(errs)), action, "", "")
		if !hasErr || cont {
			return e.finishNode(t, def, curr, action, shared, t.StepCount+1, nil)
		}
		return e.finishNode(t, def, curr, action, shared, t.StepCount+1, errorString("parallel error"))
	}

	// Launch execution based on mode
	mode := pl["mode"].(string)
	if mode == "concurrent" {
		max := node.MaxParallel
		if max <= 0 || max > len(remaining) {
			max = len(remaining)
		}
		toRun := remaining[:max]
		e.logf("task=%s node=%s parallel launch=%d", t.ID, curr, len(toRun))
		type br struct {
			svc  string
			res  interface{}
			wid  string
			wurl string
			err  error
		}
		ch := make(chan br, len(toRun))
		for _, sname := range toRun {
			go func(sv string) {
				use := DefNode{Service: sv, ExecType: node.ExecType, Func: node.Func, Script: node.Script, WeightedByLoad: node.WeightedByLoad, MaxAttempts: node.MaxAttempts, AttemptDelayMillis: node.AttemptDelayMillis}
				callParams := map[string]interface{}{}
				for k, v := range params {
					callParams[k] = v
				}
				if sp, ok := specs[sv]; ok {
					if sp.ExecType != "" {
						use.ExecType = sp.ExecType
					}
					if sp.Func != "" {
						use.Func = sp.Func
					}
					if sp.Script.Cmd != "" {
						use.Script = sp.Script
					}
					if sp.Params != nil {
						for k, v := range sp.Params {
							callParams[k] = v
						}
					}
				}
				r, wid, wurl, er := e.execExecutor(t, use, curr, input, callParams)
				ch <- br{svc: sv, res: r, wid: wid, wurl: wurl, err: er}
			}(sname)
		}
		hadErr := false
		hasPending := false
		for i := 0; i < len(toRun); i++ {
			it := <-ch

			if it.err == ErrAsyncPending {
				hasPending = true
				e.logf("task=%s node=%s branch=%s status=pending_queue", t.ID, curr, it.svc)
				// Do not record run as error, just skip
				continue
			}

			e.logf("task=%s node=%s branch=%s status=%s error=%v", t.ID, curr, it.svc, ternary(it.err == nil, "ok", "error"), it.err)
			e.recordRun(t, curr, 1, ternary(it.err == nil, "ok", "error"), map[string]interface{}{"input_key": node.Prep.InputKey, "branch": it.svc}, input, it.res, errString(it.err), "", it.wid, it.wurl)
			if it.err != nil {
				hadErr = true
				errs[it.svc] = it.err.Error()
			} else {
				done[it.svc] = it.res
			}
		}
		pl["done"] = done
		pl["errs"] = errs
		rt[key] = pl
		shared["_rt"] = rt

		if hasPending {
			// If any branch is pending, suspend the task
			return e.suspendTask(t, "waiting_queue", shared)
		}

		strat := node.FailureStrategy
		if strat == "fail_fast" && hadErr {
			e.logf("task=%s node=%s fail_fast errors=%d", t.ID, curr, len(errs))
			agg := make([]interface{}, 0, len(done))
			for _, sname := range svcs {
				if v, ok := done[sname]; ok {
					agg = append(agg, v)
				}
			}
			action := ""
			if node.Post.OutputKey != "" {
				shared[node.Post.OutputKey] = agg
			}
			if node.Post.ActionStatic != "" {
				action = node.Post.ActionStatic
			} else if node.Post.ActionKey != "" {
				action = pickAction(map[string]interface{}{"result": agg}, node.Post.ActionKey)
			}
			next := findNext(def.Edges, curr, action)
			if e.Owner != "" {
				_ = e.Store.UpdateTaskStatusOwned(t.ID, e.Owner, ternary(next == "", "failed", "running"))
			} else {
				_ = e.Store.UpdateTaskStatus(t.ID, ternary(next == "", "failed", "running"))
			}
			if e.Owner != "" {
				_ = e.Store.UpdateTaskProgressOwned(t.ID, e.Owner, next, action, toJSON(shared), t.StepCount+1)
			} else {
				_ = e.Store.UpdateTaskProgress(t.ID, next, action, toJSON(shared), t.StepCount+1)
			}
			return nil
		}
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
	var nextSvc string
	for _, sname := range svcs {
		if _, ok := done[sname]; !ok {
			nextSvc = sname
			break
		}
	}
	e.logf("task=%s node=%s parallel next=%s", t.ID, curr, nextSvc)
	use := DefNode{Service: nextSvc, ExecType: node.ExecType, Func: node.Func, Script: node.Script, WeightedByLoad: node.WeightedByLoad, MaxAttempts: node.MaxAttempts, AttemptDelayMillis: node.AttemptDelayMillis}
	callParams := map[string]interface{}{}
	for k, v := range params {
		callParams[k] = v
	}
	if sp, ok := specs[nextSvc]; ok {
		if sp.ExecType != "" {
			use.ExecType = sp.ExecType
		}
		if sp.Func != "" {
			use.Func = sp.Func
		}
		if sp.Script.Cmd != "" {
			use.Script = sp.Script
		}
		if sp.Params != nil {
			for k, v := range sp.Params {
				callParams[k] = v
			}
		}
	}
	execRes, workerID, workerURL, execErr := e.execExecutor(t, use, curr, input, callParams)
	e.recordRun(t, curr, 1, ternary(execErr == nil, "ok", "error"), map[string]interface{}{"input_key": node.Prep.InputKey, "branch": nextSvc}, input, execRes, errString(execErr), "", workerID, workerURL)
	if execErr != nil {
		if execErr == ErrAsyncPending {
			return e.suspendTask(t, "waiting_queue", shared)
		}
		errs[nextSvc] = errString(execErr)
		pl["errs"] = errs
		rt[key] = pl
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
	done[nextSvc] = execRes
	pl["done"] = done
	rt[key] = pl
	shared["_rt"] = rt
	_ = e.Store.UpdateTaskStatus(t.ID, "running")
	_ = e.Store.UpdateTaskProgress(t.ID, curr, "", toJSON(shared), t.StepCount+1)
	return nil
}
