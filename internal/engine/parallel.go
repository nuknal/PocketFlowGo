package engine

import (
	"github.com/nuknal/PocketFlowGo/internal/store"
)

// runParallel executes multiple services in parallel (concurrently or sequentially).
func (e *Engine) runParallel(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, params map[string]interface{}, input interface{}) error {
	svcs, specs := e.resolveParallelServices(node, params)

	// Handle no services case
	if len(svcs) == 0 {
		return e.handleNoServices(t, curr, node, input, shared)
	}

	// Initialize runtime state for parallel execution
	rt, pl, done, errs := e.initParallelState(t, curr, shared, node)
	key := "pl:" + curr

	// Determine remaining services
	remaining := e.getRemainingServices(svcs, done, errs)
	e.logf("task=%s node=%s kind=parallel mode=%s remaining=%d total=%d", t.ID, curr, pl["mode"], len(remaining), len(svcs))

	// If all completed, aggregate results and finish
	if len(remaining) == 0 {
		return e.finishParallelNode(t, def, node, curr, shared, input, svcs, done, errs, rt, key)
	}

	// Launch execution based on mode
	mode := pl["mode"].(string)
	if mode == "concurrent" {
		return e.runConcurrent(t, def, node, curr, shared, params, input, svcs, specs, remaining, pl, done, errs, rt, key)
	}

	// Sequential mode
	return e.runSequential(t, def, node, curr, shared, params, input, svcs, specs, pl, done, errs, rt, key, remaining)
}

// resolveParallelServices determines the list of services to execute and their specs
func (e *Engine) resolveParallelServices(node DefNode, params map[string]interface{}) ([]string, map[string]ExecSpec) {
	svcs := node.ParallelServices
	specs := map[string]ExecSpec{}

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
	return svcs, specs
}

// handleNoServices handles the case where no services are resolved
func (e *Engine) handleNoServices(t store.Task, curr string, node DefNode, input interface{}, shared map[string]interface{}) error {
	e.recordRun(t, curr, 1, "error", map[string]interface{}{"input_key": node.Prep.InputKey}, input, nil, "no services", "", "", "")
	e.updateTaskRunning(t, curr, shared)
	return nil
}

// initParallelState initializes or retrieves the runtime state for parallel execution
func (e *Engine) initParallelState(t store.Task, curr string, shared map[string]interface{}, node DefNode) (map[string]interface{}, map[string]interface{}, map[string]interface{}, map[string]interface{}) {
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
	return rt, pl, done, errs
}

// getRemainingServices filters out completed services (both success and error)
func (e *Engine) getRemainingServices(svcs []string, done map[string]interface{}, errs map[string]interface{}) []string {
	remaining := []string{}
	for _, sname := range svcs {
		_, isDone := done[sname]
		_, isErr := errs[sname]
		if !isDone && !isErr {
			remaining = append(remaining, sname)
		}
	}
	return remaining
}

// finishParallelNode aggregates results and finalizes the node execution
func (e *Engine) finishParallelNode(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, input interface{}, svcs []string, done map[string]interface{}, errs map[string]interface{}, rt map[string]interface{}, key string) error {
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

	// Clean up runtime state
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

// runConcurrent executes services concurrently
func (e *Engine) runConcurrent(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, params map[string]interface{}, input interface{}, svcs []string, specs map[string]ExecSpec, remaining []string, pl map[string]interface{}, done map[string]interface{}, errs map[string]interface{}, rt map[string]interface{}, key string) error {
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
			use, callParams := e.prepareExecution(node, specs, sv, params)
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
			continue
		}

		e.logf("task=%s node=%s branch=%s status=%s error=%v", t.ID, curr, it.svc, ternary(it.err == nil, "ok", "error"), it.err)
		e.recordRunDetailed(t, curr, 1, ternary(it.err == nil, "ok", "error"), "branch_complete", it.svc, map[string]interface{}{"input_key": node.Prep.InputKey, "branch": it.svc}, input, it.res, errString(it.err), "", it.wid, it.wurl)

		if it.err != nil {
			hadErr = true
			errs[it.svc] = it.err.Error()
		} else {
			done[it.svc] = it.res
		}
	}

	// Update state
	pl["done"] = done
	pl["errs"] = errs
	rt[key] = pl
	shared["_rt"] = rt

	if hasPending {
		return e.suspendTask(t, "waiting_queue", shared)
	}

	strat := node.FailureStrategy
	if strat == "fail_fast" && hadErr {
		return e.handleFailFast(t, def, node, curr, shared, svcs, done, errs)
	}

	e.updateTaskRunning(t, curr, shared)
	return nil
}

// runSequential executes services sequentially
func (e *Engine) runSequential(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, params map[string]interface{}, input interface{}, svcs []string, specs map[string]ExecSpec, pl map[string]interface{}, done map[string]interface{}, errs map[string]interface{}, rt map[string]interface{}, key string, remaining []string) error {
	if len(remaining) == 0 {
		return nil
	}
	nextSvc := remaining[0]

	e.logf("task=%s node=%s parallel next=%s", t.ID, curr, nextSvc)

	use, callParams := e.prepareExecution(node, specs, nextSvc, params)
	execRes, workerID, workerURL, execErr := e.execExecutor(t, use, curr, input, callParams)

	e.recordRunDetailed(t, curr, 1, ternary(execErr == nil, "ok", "error"), "branch_complete", nextSvc, map[string]interface{}{"input_key": node.Prep.InputKey, "branch": nextSvc}, input, execRes, errString(execErr), "", workerID, workerURL)

	if execErr != nil {
		if execErr == ErrAsyncPending {
			return e.suspendTask(t, "waiting_queue", shared)
		}

		errs[nextSvc] = errString(execErr)
		pl["errs"] = errs
		rt[key] = pl
		shared["_rt"] = rt

		e.updateTaskRunning(t, curr, shared)
		return nil
	}

	done[nextSvc] = execRes
	pl["done"] = done
	rt[key] = pl
	shared["_rt"] = rt

	e.updateTaskRunning(t, curr, shared)
	return nil
}

// prepareExecution creates the DefNode and params for a specific service execution
func (e *Engine) prepareExecution(node DefNode, specs map[string]ExecSpec, svc string, params map[string]interface{}) (DefNode, map[string]interface{}) {
	use := DefNode{Service: svc, ExecType: node.ExecType, Func: node.Func, Script: node.Script, WeightedByLoad: node.WeightedByLoad, MaxAttempts: node.MaxAttempts, AttemptDelayMillis: node.AttemptDelayMillis}
	callParams := map[string]interface{}{}
	for k, v := range params {
		callParams[k] = v
	}
	if sp, ok := specs[svc]; ok {
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
	return use, callParams
}

// updateTaskRunning updates the task status to running and saves progress
func (e *Engine) updateTaskRunning(t store.Task, curr string, shared map[string]interface{}) {
	if e.Owner != "" {
		_ = e.Store.UpdateTaskStatusOwned(t.ID, e.Owner, "running")
		_ = e.Store.UpdateTaskProgressOwned(t.ID, e.Owner, curr, "", toJSON(shared), t.StepCount+1)
	} else {
		_ = e.Store.UpdateTaskStatus(t.ID, "running")
		_ = e.Store.UpdateTaskProgress(t.ID, curr, "", toJSON(shared), t.StepCount+1)
	}
}

// handleFailFast handles the fail_fast strategy logic
func (e *Engine) handleFailFast(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, svcs []string, done map[string]interface{}, errs map[string]interface{}) error {
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

	status := ternary(next == "", "failed", "running")
	if e.Owner != "" {
		_ = e.Store.UpdateTaskStatusOwned(t.ID, e.Owner, status)
		_ = e.Store.UpdateTaskProgressOwned(t.ID, e.Owner, next, action, toJSON(shared), t.StepCount+1)
	} else {
		_ = e.Store.UpdateTaskStatus(t.ID, status)
		_ = e.Store.UpdateTaskProgress(t.ID, next, action, toJSON(shared), t.StepCount+1)
	}
	return nil
}
