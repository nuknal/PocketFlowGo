package engine

import (
	"strings"

	"github.com/nuknal/PocketFlowGo/internal/store"
)

func (e *Engine) runSubflow(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, params map[string]interface{}, input interface{}) error {
	rt, _ := shared["_rt"].(map[string]interface{})
	if rt == nil {
		rt = map[string]interface{}{}
	}
	key := "sf:" + curr
	sf, _ := rt[key].(map[string]interface{})
	if sf == nil {
		sf = map[string]interface{}{"curr": node.Subflow.Start, "shared": map[string]interface{}{}, "last": ""}
	}
	currSub, _ := sf["curr"].(string)
	subShared, _ := sf["shared"].(map[string]interface{})
	if currSub == "" {
		action := node.Post.ActionStatic
		e.recordRun(t, curr, 1, "ok", map[string]interface{}{"input_key": node.Prep.InputKey}, nil, nil, "", action, "", "")
		next := findNext(def.Edges, curr, action)
		if next == "" {
			_ = e.Store.UpdateTaskStatus(t.ID, "completed")
		} else {
			_ = e.Store.UpdateTaskStatus(t.ID, "running")
		}
		_ = e.Store.UpdateTaskProgress(t.ID, next, action, toJSON(shared), t.StepCount+1)
		e.logf("task=%s node=%s kind=subflow complete action=%s next=%s", t.ID, curr, action, next)
		return nil
	}
	e.logf("task=%s node=%s kind=subflow sub=%s", t.ID, curr, currSub)
	sn := node.Subflow.Nodes[currSub]
	var subInput interface{}
	if sn.Prep.InputMap != nil {
		m := make(map[string]interface{})
		for k, path := range sn.Prep.InputMap {
			if strings.HasPrefix(path, "$params.") {
				kk := strings.TrimPrefix(path, "$params.")
				m[k] = params[kk]
			} else {
				m[k] = subShared[path]
			}
		}
		subInput = m
	} else if sn.Prep.InputKey != "" {
		if strings.HasPrefix(sn.Prep.InputKey, "$params.") {
			k := strings.TrimPrefix(sn.Prep.InputKey, "$params.")
			subInput = params[k]
		} else {
			subInput = subShared[sn.Prep.InputKey]
		}
	}
	execRes, workerID, workerURL, execErr := e.execExecutor(sn, subInput, params)
	subAction := ""
	if execErr == nil {
		if sn.Post.OutputMap != nil {
			if mm, ok := execRes.(map[string]interface{}); ok {
				for toKey, fromField := range sn.Post.OutputMap {
					subShared[toKey] = mm[fromField]
				}
			}
		}
		if sn.Post.OutputKey != "" {
			subShared[sn.Post.OutputKey] = execRes
		}
		if sn.Post.ActionStatic != "" {
			subAction = sn.Post.ActionStatic
		} else if sn.Post.ActionKey != "" {
			subAction = pickAction(execRes, sn.Post.ActionKey)
		}
	}
	e.logf("task=%s node=%s kind=subflow sub=%s status=%s action=%s", t.ID, curr, currSub, ternary(execErr == nil, "ok", "error"), subAction)
	e.recordRun(t, curr, 1, ternary(execErr == nil, "ok", "error"), map[string]interface{}{"input_key": sn.Prep.InputKey, "sub": currSub}, subInput, execRes, errString(execErr), subAction, workerID, workerURL)
	if execErr != nil {
		_ = e.Store.UpdateTaskStatus(t.ID, "running")
		_ = e.Store.UpdateTaskProgress(t.ID, curr, "", toJSON(shared), t.StepCount+1)
		return nil
	}
	nextSub := findNext(node.Subflow.Edges, currSub, subAction)
	if nextSub == "" {
		action := ""
		if node.Post.OutputKey != "" {
			shared[node.Post.OutputKey] = subShared
		}
		if node.Post.ActionStatic != "" {
			action = node.Post.ActionStatic
		} else if node.Post.ActionKey != "" {
			action = pickAction(subShared, node.Post.ActionKey)
		}
		delete(rt, key)
		if len(rt) == 0 {
			delete(shared, "_rt")
		} else {
			shared["_rt"] = rt
		}
		next := findNext(def.Edges, curr, action)
		if next == "" {
			_ = e.Store.UpdateTaskStatus(t.ID, "completed")
		} else {
			_ = e.Store.UpdateTaskStatus(t.ID, "running")
		}
		_ = e.Store.UpdateTaskProgress(t.ID, next, action, toJSON(shared), t.StepCount+1)
		e.logf("task=%s node=%s kind=subflow finish action=%s next=%s", t.ID, curr, action, next)
		return nil
	}
	sf["curr"] = nextSub
	sf["shared"] = subShared
	rt[key] = sf
	shared["_rt"] = rt
	_ = e.Store.UpdateTaskStatus(t.ID, "running")
	_ = e.Store.UpdateTaskProgress(t.ID, curr, "", toJSON(shared), t.StepCount+1)
	return nil
}
