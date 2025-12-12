package engine

import (
	"fmt"

	"github.com/nuknal/PocketFlowGo/internal/store"
)

// runForeach executes a 'foreach' node, iterating over a list of items.
// It supports sequential or concurrent execution modes.
func (e *Engine) runForeach(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, params map[string]interface{}, input interface{}) error {
	items := e.resolveItems(input)

	// Handle empty input list
	if len(items) == 0 {
		return e.handleEmptyList(t, def, curr, node, input, shared)
	}

	// Initialize runtime state for iteration
	rt, fe, done, errs := e.initForeachState(t, curr, shared, node)
	key := "fe:" + curr

	// Determine remaining items to process
	remaining := e.getRemainingItems(items, done, errs)

	// If all items processed, aggregate results and finish
	if len(remaining) == 0 {
		return e.finishForeachNode(t, def, node, curr, shared, input, items, done, errs, rt, key)
	}

	// Process remaining items based on execution mode
	mode := fe["mode"].(string)
	if mode == "concurrent" {
		return e.runForeachConcurrent(t, def, node, curr, shared, params, input, items, remaining, fe, done, errs, rt, key)
	}

	// Sequential mode
	return e.runForeachSequential(t, def, node, curr, shared, params, input, items, remaining, fe, done, errs, rt, key)
}

// resolveItems extracts the list of items from the input
func (e *Engine) resolveItems(input interface{}) []interface{} {
	items := []interface{}{}
	if arr, ok := input.([]interface{}); ok {
		items = arr
	}
	return items
}

// handleEmptyList handles the case where the input list is empty
func (e *Engine) handleEmptyList(t store.Task, def FlowDef, curr string, node DefNode, input interface{}, shared map[string]interface{}) error {
	e.recordRun(t, curr, 1, "ok", map[string]interface{}{"input_key": node.Prep.InputKey}, input, []interface{}{}, "", node.Post.ActionStatic, "", "")
	return e.finishNode(t, def, curr, node.Post.ActionStatic, shared, t.StepCount+1, nil)
}

// initForeachState initializes or retrieves the runtime state for foreach execution
func (e *Engine) initForeachState(t store.Task, curr string, shared map[string]interface{}, node DefNode) (map[string]interface{}, map[string]interface{}, map[string]interface{}, map[string]interface{}) {
	rt, _ := shared["_rt"].(map[string]interface{})
	if rt == nil {
		rt = map[string]interface{}{}
	}
	key := "fe:" + curr
	fe, _ := rt[key].(map[string]interface{})
	if fe == nil {
		fe = map[string]interface{}{"done": map[string]interface{}{}, "errs": map[string]interface{}{}, "idx": 0, "mode": node.ParallelMode, "max": node.MaxParallel, "strategy": node.FailureStrategy}
	}
	done := fe["done"].(map[string]interface{})
	errs := fe["errs"].(map[string]interface{})
	return rt, fe, done, errs
}

// getRemainingItems finds indices of items that haven't been processed yet
func (e *Engine) getRemainingItems(items []interface{}, done map[string]interface{}, errs map[string]interface{}) []int {
	remaining := []int{}
	for i := range items {
		key := indexKey(i)
		_, isDone := done[key]
		_, isErr := errs[key]
		if !isDone && !isErr {
			remaining = append(remaining, i)
		}
	}
	return remaining
}

// finishForeachNode aggregates results and transitions to the next node
func (e *Engine) finishForeachNode(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, input interface{}, items []interface{}, done map[string]interface{}, errs map[string]interface{}, rt map[string]interface{}, key string) error {
	agg := make([]interface{}, 0, len(items))
	for i := range items {
		agg = append(agg, done[indexKey(i)])
	}
	action := node.Post.ActionStatic
	if action == "" && node.Post.ActionKey != "" {
		action = pickAction(map[string]interface{}{"result": agg}, node.Post.ActionKey)
	}
	if node.Post.OutputKey != "" {
		shared[node.Post.OutputKey] = agg
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
	return e.finishNode(t, def, curr, action, shared, t.StepCount+1, errorString("foreach error"))
}

// runForeachConcurrent executes items concurrently
func (e *Engine) runForeachConcurrent(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, params map[string]interface{}, input interface{}, items []interface{}, remaining []int, fe map[string]interface{}, done map[string]interface{}, errs map[string]interface{}, rt map[string]interface{}, key string) error {
	max := node.MaxParallel
	if max <= 0 || max > len(remaining) {
		max = len(remaining)
	}
	sel := remaining[:max]

	type br struct {
		idx  int
		res  interface{}
		wid  string
		wurl string
		err  error
	}
	ch := make(chan br, len(sel))

	// Launch concurrent goroutines
	for _, i := range sel {
		go func(ii int, it interface{}) {
			use, callParams := e.prepareForeachExecution(node, ii, params)
			r, wid, wurl, er := e.execExecutor(t, use, curr, it, callParams)
			ch <- br{idx: ii, res: r, wid: wid, wurl: wurl, err: er}
		}(i, items[i])
	}

	hadErr := false
	hasPending := false
	for i := 0; i < len(sel); i++ {
		it := <-ch

		if it.err == ErrAsyncPending {
			hasPending = true
			e.logf("task=%s node=%s branch=%d status=pending_queue", t.ID, curr, it.idx)
			continue
		}

		e.recordRunDetailed(t, curr, 1, ternary(it.err == nil, "ok", "error"), "item_complete", fmt.Sprintf("%d", it.idx), map[string]interface{}{"branch": it.idx}, items[it.idx], it.res, errString(it.err), "", it.wid, it.wurl)
		if it.err != nil {
			hadErr = true
			errs[indexKey(it.idx)] = it.err.Error()
		} else {
			done[indexKey(it.idx)] = it.res
		}
	}

	fe["done"] = done
	fe["errs"] = errs
	rt[key] = fe
	shared["_rt"] = rt

	if hasPending {
		return e.suspendTask(t, "waiting_queue", shared)
	}

	if node.FailureStrategy == "fail_fast" && hadErr {
		return e.handleForeachFailFast(t, def, node, curr, shared, items, done, errs)
	}

	e.updateTaskRunning(t, curr, shared)
	return nil
}

// runForeachSequential executes items sequentially
func (e *Engine) runForeachSequential(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, params map[string]interface{}, input interface{}, items []interface{}, remaining []int, fe map[string]interface{}, done map[string]interface{}, errs map[string]interface{}, rt map[string]interface{}, key string) error {
	if len(remaining) == 0 {
		return nil
	}
	idx := remaining[0]

	use, callParams := e.prepareForeachExecution(node, idx, params)
	execRes, workerID, workerURL, execErr := e.execExecutor(t, use, curr, items[idx], callParams)

	e.recordRunDetailed(t, curr, 1, ternary(execErr == nil, "ok", "error"), "item_complete", fmt.Sprintf("%d", idx), map[string]interface{}{"branch": idx}, items[idx], execRes, errString(execErr), "", workerID, workerURL)

	if execErr != nil {
		if execErr == ErrAsyncPending {
			return e.suspendTask(t, "waiting_queue", shared)
		}
		errs[indexKey(idx)] = errString(execErr)
		fe["errs"] = errs
		rt[key] = fe
		shared["_rt"] = rt

		e.updateTaskRunning(t, curr, shared)
		return nil
	}

	done[indexKey(idx)] = execRes
	fe["done"] = done
	rt[key] = fe
	shared["_rt"] = rt

	e.updateTaskRunning(t, curr, shared)
	return nil
}

// prepareForeachExecution creates the DefNode and params for a specific iteration
func (e *Engine) prepareForeachExecution(node DefNode, idx int, params map[string]interface{}) (DefNode, map[string]interface{}) {
	use := DefNode{Service: node.Service, ExecType: node.ExecType, Func: node.Func, Script: node.Script, WeightedByLoad: node.WeightedByLoad, MaxAttempts: node.MaxAttempts, AttemptDelayMillis: node.AttemptDelayMillis}

	// Find spec for this index if exists
	var sp ExecSpec
	found := false
	for _, s := range node.ForeachExecs {
		if s.Index == idx {
			sp = s
			found = true
			break
		}
	}

	callParams := map[string]interface{}{}
	for k, v := range params {
		callParams[k] = v
	}

	if found {
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

// handleForeachFailFast handles the fail_fast strategy logic for foreach
func (e *Engine) handleForeachFailFast(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, items []interface{}, done map[string]interface{}, errs map[string]interface{}) error {
	agg := make([]interface{}, 0, len(items))
	for i := range items {
		if v, ok := done[indexKey(i)]; ok {
			agg = append(agg, v)
		}
	}
	action := node.Post.ActionStatic
	if action == "" && node.Post.ActionKey != "" {
		action = pickAction(map[string]interface{}{"result": agg}, node.Post.ActionKey)
	}
	if node.Post.OutputKey != "" {
		shared[node.Post.OutputKey] = agg
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
