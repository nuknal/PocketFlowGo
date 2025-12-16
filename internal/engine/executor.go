package engine

import (
	"time"
)

// runExecutorNode executes a node of kind 'executor'.
// It handles retries, input/output mapping, and transitions.
func (e *Engine) runExecutorNode(in NodeRunInput) error {
	var execRes interface{}
	var workerID, workerURL, logPath string
	var execErr error
	action := ""
	attempts := 0

	// Loop for retries
	for {
		attempts++
		// Execute the logic
		execIn := ExecutorInput{
			Task:    in.Task,
			Node:    in.Node,
			NodeKey: in.NodeKey,
			Input:   in.Input,
			Params:  in.Params,
		}
		var res ExecutorResult
		res = e.execExecutor(execIn)

		execRes = res.Result
		workerID = res.WorkerID
		workerURL = res.WorkerURL
		logPath = res.LogPath
		execErr = res.Error

		// Handle Async Queue suspension
		if execErr == ErrAsyncPending {
			return e.suspendTask(in.Task, "waiting_queue", in.Shared)
		}

		// Log and record execution attempt
		e.logf("task=%s node=%s kind=executor attempt=%d worker=%s status=%s", in.Task.ID, in.NodeKey, attempts, workerID, ternary(execErr == nil, "ok", "error"))
		if !res.SkipRecord {
			e.recordRun(in.Task, in.NodeKey, attempts, ternary(execErr == nil, "ok", "error"), map[string]interface{}{"input_key": in.Node.Prep.InputKey}, in.Input, execRes, errString(execErr), action, workerID, workerURL, logPath)
		}

		if execErr == nil {
			break
		}

		// Check retry limits
		if execErr == ErrFatal || attempts > in.Node.MaxRetries {
			break
		}

		// Wait before retry
		if in.Node.WaitMillis > 0 {
			time.Sleep(time.Duration(in.Node.WaitMillis) * time.Millisecond)
		}
	}

	// If execution succeeded, handle outputs and determine next action
	if execErr == nil {
		if in.Node.Post.OutputMap != nil {
			if mm, ok := execRes.(map[string]interface{}); ok {
				for toKey, fromField := range in.Node.Post.OutputMap {
					in.Shared[toKey] = mm[fromField]
				}
			}
		}
		if in.Node.Post.OutputKey != "" {
			in.Shared[in.Node.Post.OutputKey] = execRes
		}

		// Determine transition
		if in.Node.Post.ActionStatic != "" {
			action = in.Node.Post.ActionStatic
		} else if in.Node.Post.ActionKey != "" {
			action = pickAction(execRes, in.Node.Post.ActionKey)
		}
	}
	return e.finishNode(in.Task, in.FlowDef, in.NodeKey, action, in.Shared, in.Task.StepCount+1, execErr)
}

// execExecutor dispatches execution to the appropriate handler based on ExecType.
func (e *Engine) execExecutor(in ExecutorInput) ExecutorResult {
	et := in.Node.ExecType
	if et == "" {
		et = "http"
	}
	switch et {
	case "http":
		return e.execHTTP(in)
	case "local_func":
		return e.execLocalFunc(in)
	case "local_script":
		return e.execLocalScript(in)
	case "queue":
		return e.execQueue(in)
	default:
		return ExecutorResult{Error: errorString("unsupported exec")}
	}
}
