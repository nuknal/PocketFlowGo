package engine

import (
	"github.com/nuknal/PocketFlowGo/internal/store"
)

func (e *Engine) runForeach(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, params map[string]interface{}, input interface{}) error {
	items := []interface{}{}
	if arr, ok := input.([]interface{}); ok {
		items = arr
	}
	if len(items) == 0 {
		e.recordRun(t, curr, 1, "ok", map[string]interface{}{"input_key": node.Prep.InputKey}, input, []interface{}{}, "", node.Post.ActionStatic, "", "")
		return e.finishNode(t, def, curr, node.Post.ActionStatic, shared, t.StepCount+1, nil)
	}
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
	remaining := []int{}
	for i := range items {
		if _, ok := done[indexKey(i)]; !ok {
			remaining = append(remaining, i)
		}
	}
	if len(remaining) == 0 {
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
	mode := fe["mode"].(string)
	if mode == "concurrent" {
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
		specByIdx := map[int]ExecSpec{}
		for _, sp := range node.ForeachExecs {
			specByIdx[sp.Index] = sp
		}
		for _, i := range sel {
			go func(ii int, it interface{}) {
				use := DefNode{Service: node.Service, ExecType: node.ExecType, Func: node.Func, Script: node.Script, WeightedByLoad: node.WeightedByLoad, MaxAttempts: node.MaxAttempts, AttemptDelayMillis: node.AttemptDelayMillis}
				callParams := map[string]interface{}{}
				for k, v := range params {
					callParams[k] = v
				}
				if sp, ok := specByIdx[ii]; ok {
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
				r, wid, wurl, er := e.execExecutor(use, it, callParams)
				ch <- br{idx: ii, res: r, wid: wid, wurl: wurl, err: er}
			}(i, items[i])
		}
		hadErr := false
		for i := 0; i < len(sel); i++ {
			it := <-ch
			e.recordRun(t, curr, 1, ternary(it.err == nil, "ok", "error"), map[string]interface{}{"branch": it.idx}, items[it.idx], it.res, errString(it.err), "", it.wid, it.wurl)
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
		if node.FailureStrategy == "fail_fast" && hadErr {
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
	idx := -1
	for _, i := range remaining {
		idx = i
		break
	}
	use := DefNode{Service: node.Service, ExecType: node.ExecType, Func: node.Func, Script: node.Script, WeightedByLoad: node.WeightedByLoad, MaxAttempts: node.MaxAttempts, AttemptDelayMillis: node.AttemptDelayMillis}
	specByIdx := map[int]ExecSpec{}
	for _, sp := range node.ForeachExecs {
		specByIdx[sp.Index] = sp
	}
	callParams := map[string]interface{}{}
	for k, v := range params {
		callParams[k] = v
	}
	if sp, ok := specByIdx[idx]; ok {
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
	execRes, workerID, workerURL, execErr := e.execExecutor(use, items[idx], callParams)
	e.recordRun(t, curr, 1, ternary(execErr == nil, "ok", "error"), map[string]interface{}{"branch": idx}, items[idx], execRes, errString(execErr), "", workerID, workerURL)
	if execErr != nil {
		errs[indexKey(idx)] = errString(execErr)
		fe["errs"] = errs
		rt[key] = fe
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
	done[indexKey(idx)] = execRes
	fe["done"] = done
	rt[key] = fe
	shared["_rt"] = rt
	_ = e.Store.UpdateTaskStatus(t.ID, "running")
	_ = e.Store.UpdateTaskProgress(t.ID, curr, "", toJSON(shared), t.StepCount+1)
	return nil
}
