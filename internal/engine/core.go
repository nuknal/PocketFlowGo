package engine

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nuknal/PocketFlowGo/internal/store"
)

type Engine struct {
	Store      *store.SQLite
	HTTP       *http.Client
	Log        *log.Logger
	Owner      string
	LocalFuncs map[string]func(context.Context, interface{}, map[string]interface{}) (interface{}, error)
}

func New(s *store.SQLite) *Engine {
	return &Engine{Store: s, HTTP: &http.Client{}, Log: log.Default(), Owner: "", LocalFuncs: map[string]func(context.Context, interface{}, map[string]interface{}) (interface{}, error){}}
}

func (e *Engine) RegisterFunc(name string, fn func(context.Context, interface{}, map[string]interface{}) (interface{}, error)) {
	if e.LocalFuncs == nil {
		e.LocalFuncs = map[string]func(context.Context, interface{}, map[string]interface{}) (interface{}, error){}
	}
	e.LocalFuncs[name] = fn
}

func (e *Engine) logf(format string, args ...interface{}) {
	if e.Log != nil {
		e.Log.Printf(format, args...)
	}
}

func (e *Engine) buildInput(node DefNode, shared map[string]interface{}, params map[string]interface{}) interface{} {
	if node.Prep.InputMap != nil {
		m := make(map[string]interface{})
		for k, path := range node.Prep.InputMap {
			if strings.HasPrefix(path, "$") {
				m[k] = resolveRef(path, shared, params, nil)
			} else {
				m[k] = getByPath(shared, path)
			}
		}
		return m
	}
	if node.Prep.InputKey != "" {
		if strings.HasPrefix(node.Prep.InputKey, "$") {
			return resolveRef(node.Prep.InputKey, shared, params, nil)
		}
		return getByPath(shared, node.Prep.InputKey)
	}
	return nil
}

func (e *Engine) cancelTask(t store.Task) error {
	shared := map[string]interface{}{}
	_ = json.Unmarshal([]byte(t.SharedJSON), &shared)
	if e.Owner != "" {
		_ = e.Store.UpdateTaskStatusOwned(t.ID, e.Owner, "canceled")
		_ = e.Store.UpdateTaskProgressOwned(t.ID, e.Owner, "", "canceled", toJSON(shared), t.StepCount)
	} else {
		_ = e.Store.UpdateTaskStatus(t.ID, "canceled")
		_ = e.Store.UpdateTaskProgress(t.ID, "", "canceled", toJSON(shared), t.StepCount)
	}
	e.logf("task=%s canceled node=%s", t.ID, t.CurrentNodeKey)
	nr := map[string]interface{}{
		"task_id":          t.ID,
		"node_key":         t.CurrentNodeKey,
		"attempt_no":       0,
		"status":           "canceled",
		"prep_json":        toJSON(map[string]interface{}{}),
		"exec_input_json":  toJSON(nil),
		"exec_output_json": toJSON(nil),
		"error_text":       "",
		"action":           "canceled",
		"started_at":       time.Now().Unix(),
		"finished_at":      time.Now().Unix(),
		"worker_id":        "",
		"worker_url":       "",
	}
	return e.Store.SaveNodeRun(nr)
}

func (e *Engine) recordRun(t store.Task, curr string, attempt int, status string, prep map[string]interface{}, input interface{}, output interface{}, errText string, action string, workerID string, workerURL string) {
	nr := map[string]interface{}{
		"task_id":          t.ID,
		"node_key":         curr,
		"attempt_no":       attempt,
		"status":           status,
		"prep_json":        toJSON(prep),
		"exec_input_json":  toJSON(input),
		"exec_output_json": toJSON(output),
		"error_text":       errText,
		"action":           action,
		"started_at":       time.Now().Unix(),
		"finished_at":      time.Now().Unix(),
		"worker_id":        workerID,
		"worker_url":       workerURL,
	}
	_ = e.Store.SaveNodeRun(nr)
}

func (e *Engine) finishNode(t store.Task, def FlowDef, curr string, action string, shared map[string]interface{}, stepCount int, execErr error) error {
	next := findNext(def.Edges, curr, action)
	st := ternary(execErr == nil, "ok", "error")
	if execErr == nil {
		if next == "" {
			if e.Owner != "" {
				_ = e.Store.UpdateTaskStatusOwned(t.ID, e.Owner, "completed")
			} else {
				_ = e.Store.UpdateTaskStatus(t.ID, "completed")
			}
		} else {
			if e.Owner != "" {
				_ = e.Store.UpdateTaskStatusOwned(t.ID, e.Owner, "running")
			} else {
				_ = e.Store.UpdateTaskStatus(t.ID, "running")
			}
		}
	} else {
		if next == "" {
			if e.Owner != "" {
				_ = e.Store.UpdateTaskStatusOwned(t.ID, e.Owner, "failed")
			} else {
				_ = e.Store.UpdateTaskStatus(t.ID, "failed")
			}
		} else {
			if e.Owner != "" {
				_ = e.Store.UpdateTaskStatusOwned(t.ID, e.Owner, "running")
			} else {
				_ = e.Store.UpdateTaskStatus(t.ID, "running")
			}
		}
	}
	if e.Owner != "" {
		_ = e.Store.UpdateTaskProgressOwned(t.ID, e.Owner, next, action, toJSON(shared), stepCount)
	} else {
		_ = e.Store.UpdateTaskProgress(t.ID, next, action, toJSON(shared), stepCount)
	}
	e.logf("task=%s node=%s finish action=%s next=%s status=%s", t.ID, curr, action, next, st)
	return nil
}

func (e *Engine) RunOnce(taskID string) error {
	t, err := e.Store.GetTask(taskID)
	if err != nil {
		return err
	}
	if e.Owner != "" {
		if t.LeaseOwner != e.Owner {
			return errorString("lease_mismatch")
		}
		if t.LeaseExpiry <= time.Now().Unix() {
			return errorString("lease_expired")
		}
	}
	if t.Status == "canceling" {
		return e.cancelTask(t)
	}
	fv, err := e.Store.GetFlowVersionByID(t.FlowVersionID)
	if err != nil {
		return err
	}
	var def FlowDef
	if err := json.Unmarshal([]byte(fv.DefinitionJSON), &def); err != nil {
		return err
	}
	curr := t.CurrentNodeKey
	node := def.Nodes[curr]
	shared := map[string]interface{}{}
	_ = json.Unmarshal([]byte(t.SharedJSON), &shared)
	params := map[string]interface{}{}
	_ = json.Unmarshal([]byte(t.ParamsJSON), &params)
	for k, v := range node.Params {
		params[k] = v
	}
	input := e.buildInput(node, shared, params)
	switch {
	case node.Kind == "choice":
		return e.runChoice(t, def, node, curr, shared, params, input)
	case node.Kind == "parallel":
		return e.runParallel(t, def, node, curr, shared, params, input)
	case node.Kind == "subflow" && node.Subflow != nil:
		return e.runSubflow(t, def, node, curr, shared, params, input)
	case node.Kind == "timer":
		return e.runTimer(t, def, node, curr, shared, params, input)
	case node.Kind == "foreach":
		return e.runForeach(t, def, node, curr, shared, params, input)
	case node.Kind == "wait_event":
		return e.runWaitEvent(t, def, node, curr, shared, params, input)
	case node.Kind == "approval":
		return e.runApproval(t, def, node, curr, shared, params, input)
	case node.Kind == "executor" || node.Kind == "remote":
		return e.runExecutorNode(t, def, node, curr, shared, params, input)
	default:
		return e.runExecutorNode(t, def, node, curr, shared, params, input)
	}
}
