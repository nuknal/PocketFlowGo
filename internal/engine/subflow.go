package engine

import (
	"strings"
	"time"

	"github.com/nuknal/PocketFlowGo/internal/store"
)

// runSubflow executes a nested flow definition.
// It manages the subflow's state and progression independently of the main flow.
func (e *Engine) runSubflow(in NodeRunInput) error {
	// Initialize runtime state for subflow
	rt, sf, currSub, subShared := e.initSubflowState(in.Task, in.NodeKey, in.Node, in.Shared)
	key := "sf:" + in.NodeKey

	// Handle retry strategy delay
	if e.handleSubflowRetryDelay(in.Task, in.NodeKey, in.Node, in.Shared, rt, sf, key) {
		return nil
	}

	// Check if subflow execution is complete
	if currSub == "" {
		return e.finishSubflow(in.Task, in.FlowDef, in.NodeKey, in.Node, in.Shared, subShared, rt, key)
	}

	// Prepare execution for the current node in subflow
	e.logf("task=%s node=%s kind=subflow sub=%s", in.Task.ID, in.NodeKey, currSub)
	sn := in.Node.Subflow.Nodes[currSub]

	// Prepare parameters and input for the sub-node
	childParams := e.prepareSubNodeParams(in.Node, sn, in.Params, currSub)
	subInput := e.prepareSubNodeInput(sn, childParams, subShared)

	// Determine execution configuration (overrides)
	eff := e.resolveSubNodeConfig(in.Node, currSub, sn)

	// Execute the sub-node
	execIn := ExecutorInput{
		Task:    in.Task,
		Node:    eff,
		NodeKey: in.NodeKey,
		Input:   subInput,
		Params:  childParams,
	}
	res := e.execExecutor(execIn)
	execRes, workerID, workerURL, logPath, execErr := res.Result, res.WorkerID, res.WorkerURL, res.LogPath, res.Error

	// Process result and determine next action
	subAction := ""
	if execErr == nil {
		subAction = e.processSubNodeSuccess(sn, execRes, subShared)
	}

	e.logf("task=%s node=%s kind=subflow sub=%s status=%s action=%s", in.Task.ID, in.NodeKey, currSub, ternary(execErr == nil, "ok", "error"), subAction)
	e.recordRunDetailed(in.Task, in.NodeKey, 1, ternary(execErr == nil, "ok", "error"), "sub_node_complete", currSub, map[string]interface{}{"input_key": sn.Prep.InputKey, "sub": currSub}, subInput, execRes, errString(execErr), subAction, workerID, workerURL, logPath)

	if execErr != nil {
		if execErr == ErrAsyncPending {
			return e.suspendTask(in.Task, "waiting_queue", in.Shared)
		}

		// Handle retry logic
		if in.Node.FailureStrategy == "retry" {
			if e.handleSubflowRetry(in.Task, in.NodeKey, in.Node, in.Shared, rt, sf, key) {
				return nil
			}
			// Retries exhausted, fall through to fail
		}

		// Handle failure completion
		return e.finishSubflowFailure(in.Task, in.FlowDef, in.Node, in.NodeKey, in.Shared, subShared, rt, key, execErr)
	}

	// Transition to next sub-node
	nextSub := findNext(in.Node.Subflow.Edges, currSub, subAction)
	if nextSub == "" {
		// Subflow reached end
		return e.finishSubflowSuccess(in.Task, in.FlowDef, in.Node, in.NodeKey, in.Shared, subShared, rt, key, subAction)
	}

	// Advance subflow state
	sf["curr"] = nextSub
	sf["shared"] = subShared
	rt[key] = sf
	in.Shared["_rt"] = rt
	e.updateTaskRunning(in.Task, in.NodeKey, in.Shared)
	return nil
}

// initSubflowState initializes or retrieves the runtime state for subflow execution
func (e *Engine) initSubflowState(t store.Task, curr string, node DefNode, shared map[string]interface{}) (map[string]interface{}, map[string]interface{}, string, map[string]interface{}) {
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
	return rt, sf, currSub, subShared
}

// handleSubflowRetryDelay checks if we need to wait for a retry delay
// Returns true if execution should pause (delay active)
func (e *Engine) handleSubflowRetryDelay(t store.Task, curr string, node DefNode, shared map[string]interface{}, rt map[string]interface{}, sf map[string]interface{}, key string) bool {
	if node.FailureStrategy != "retry" {
		return false
	}
	now := time.Now().UnixMilli()
	nt := int64(0)
	if v, ok := sf["next_try_at"].(int64); ok {
		nt = v
	} else if v2, ok := sf["next_try_at"].(float64); ok {
		nt = int64(v2)
	}
	if nt > 0 && now < nt {
		rt[key] = sf
		shared["_rt"] = rt
		e.updateTaskRunning(t, curr, shared)
		return true
	}
	return false
}

// finishSubflow handles the case where the subflow itself is complete (empty current node)
func (e *Engine) finishSubflow(t store.Task, def FlowDef, curr string, node DefNode, shared map[string]interface{}, subShared map[string]interface{}, rt map[string]interface{}, key string) error {
	action := node.Post.ActionStatic
	e.recordRun(t, curr, 1, "ok", map[string]interface{}{"input_key": node.Prep.InputKey}, nil, nil, "", action, "", "", "")

	// Clean up runtime state if needed (though typically this is done when last node finishes)
	// But here we might be re-entering a completed subflow?
	// The original logic just finished the node.

	return e.finishNode(t, def, curr, action, shared, t.StepCount+1, nil)
}

// prepareSubNodeParams merges params for the sub-node
func (e *Engine) prepareSubNodeParams(node DefNode, sn DefNode, params map[string]interface{}, currSub string) map[string]interface{} {
	childParams := map[string]interface{}{}
	for k, v := range params {
		childParams[k] = v
	}
	for k, v := range sn.Params {
		childParams[k] = v
	}
	// Apply overrides from SubflowExecs
	for _, sp := range node.SubflowExecs {
		if sp.Node == currSub {
			for k, v := range sp.Params {
				childParams[k] = v
			}
			break
		}
	}
	return childParams
}

// prepareSubNodeInput resolves input for the sub-node
func (e *Engine) prepareSubNodeInput(sn DefNode, childParams map[string]interface{}, subShared map[string]interface{}) interface{} {
	var subInput interface{}
	if sn.Prep.InputMap != nil {
		m := make(map[string]interface{})
		for k, path := range sn.Prep.InputMap {
			if strings.HasPrefix(path, "$params.") {
				kk := strings.TrimPrefix(path, "$params.")
				m[k] = childParams[kk]
			} else {
				m[k] = subShared[path]
			}
		}
		subInput = m
	} else if sn.Prep.InputKey != "" {
		if strings.HasPrefix(sn.Prep.InputKey, "$params.") {
			k := strings.TrimPrefix(sn.Prep.InputKey, "$params.")
			subInput = childParams[k]
		} else {
			subInput = subShared[sn.Prep.InputKey]
		}
	}
	return subInput
}

// resolveSubNodeConfig applies overrides and defaults for the sub-node execution
func (e *Engine) resolveSubNodeConfig(node DefNode, currSub string, sn DefNode) DefNode {
	eff := DefNode{
		Service:  sn.Service,
		ExecType: sn.ExecType,
		Func:     sn.Func,
		Script:   sn.Script,
		// Map other fields as needed, though DefNode struct might differ slightly from DefNode
	}

	// Inherit from parent node if missing in sub-node
	if eff.ExecType == "" && node.ExecType != "" {
		eff.ExecType = node.ExecType
	}
	if eff.Func == "" && node.Func != "" {
		eff.Func = node.Func
	}
	if eff.Script.Cmd == "" && node.Script.Cmd != "" {
		eff.Script = node.Script
	}

	// Apply overrides from SubflowExecs
	for _, sp := range node.SubflowExecs {
		if sp.Node == currSub {
			if sp.Service != "" {
				eff.Service = sp.Service
			}
			if sp.ExecType != "" {
				eff.ExecType = sp.ExecType
			}
			if sp.Func != "" {
				eff.Func = sp.Func
			}
			if sp.Script.Cmd != "" {
				eff.Script = sp.Script
			}
			// Note: Params override is handled in prepareSubNodeParams via merging logic if needed,
			// but here we are just configuring the definition.
			break
		}
	}
	return eff
}

// processSubNodeSuccess handles successful execution of a sub-node
func (e *Engine) processSubNodeSuccess(sn DefNode, execRes interface{}, subShared map[string]interface{}) string {
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

	subAction := ""
	if sn.Post.ActionStatic != "" {
		subAction = sn.Post.ActionStatic
	} else if sn.Post.ActionKey != "" {
		subAction = pickAction(execRes, sn.Post.ActionKey)
	}
	return subAction
}

// handleSubflowRetry manages retry logic for failed sub-nodes
// Returns true if retry is scheduled (execution should stop/return)
func (e *Engine) handleSubflowRetry(t store.Task, curr string, node DefNode, shared map[string]interface{}, rt map[string]interface{}, sf map[string]interface{}, key string) bool {
	rcount := 0
	if v, ok := sf["retries"].(int); ok {
		rcount = v
	} else if v2, ok := sf["retries"].(float64); ok {
		rcount = int(v2)
	}
	rcount++
	sf["retries"] = rcount
	if node.WaitMillis > 0 {
		sf["next_try_at"] = time.Now().UnixMilli() + int64(node.WaitMillis)
	}

	if node.MaxRetries > 0 && rcount >= node.MaxRetries {
		return false // Exhausted retries
	}

	rt[key] = sf
	shared["_rt"] = rt
	e.updateTaskRunning(t, curr, shared)
	return true
}

// finishSubflowFailure handles the final failure of a sub-node (retries exhausted or fail_fast)
func (e *Engine) finishSubflowFailure(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, subShared map[string]interface{}, rt map[string]interface{}, key string, execErr error) error {
	action := node.Post.ActionStatic
	if action == "" && node.Post.ActionKey != "" {
		action = pickAction(subShared, node.Post.ActionKey)
	}
	if node.Post.OutputKey != "" {
		shared[node.Post.OutputKey] = subShared
	}

	// Cleanup
	delete(rt, key)
	if len(rt) == 0 {
		delete(shared, "_rt")
	} else {
		shared["_rt"] = rt
	}

	if node.FailureStrategy == "continue" {
		return e.finishNode(t, def, curr, action, shared, t.StepCount+1, nil)
	}
	return e.finishNode(t, def, curr, action, shared, t.StepCount+1, execErr)
}

// finishSubflowSuccess handles the completion of the entire subflow
func (e *Engine) finishSubflowSuccess(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, subShared map[string]interface{}, rt map[string]interface{}, key string, lastSubAction string) error {
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

	e.logf("task=%s node=%s kind=subflow finish action=%s next=%s", t.ID, curr, action, "TODO") // next resolved in finishNode
	return e.finishNode(t, def, curr, action, shared, t.StepCount+1, nil)
}
