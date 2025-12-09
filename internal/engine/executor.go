package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/nuknal/PocketFlowGo/internal/store"
)

func (e *Engine) runExecutorNode(t store.Task, def FlowDef, node DefNode, curr string, shared map[string]interface{}, params map[string]interface{}, input interface{}) error {
	var execRes interface{}
	var workerID, workerURL string
	var execErr error
	action := ""
	attempts := 0
	for {
		attempts++
		execRes, workerID, workerURL, execErr = e.execExecutor(node, input, params)
		e.logf("task=%s node=%s kind=executor attempt=%d worker=%s status=%s", t.ID, curr, attempts, workerID, ternary(execErr == nil, "ok", "error"))
		e.recordRun(t, curr, attempts, ternary(execErr == nil, "ok", "error"), map[string]interface{}{"input_key": node.Prep.InputKey}, input, execRes, errString(execErr), action, workerID, workerURL)
		if execErr == nil {
			break
		}
		if attempts > node.MaxRetries {
			break
		}
		if node.WaitMillis > 0 {
			time.Sleep(time.Duration(node.WaitMillis) * time.Millisecond)
		}
	}
	if execErr == nil {
		if node.Post.OutputMap != nil {
			if mm, ok := execRes.(map[string]interface{}); ok {
				for toKey, fromField := range node.Post.OutputMap {
					shared[toKey] = mm[fromField]
				}
			}
		}
		if node.Post.OutputKey != "" {
			shared[node.Post.OutputKey] = execRes
		}
		if node.Post.ActionStatic != "" {
			action = node.Post.ActionStatic
		} else if node.Post.ActionKey != "" {
			action = pickAction(execRes, node.Post.ActionKey)
		}
	}
	return e.finishNode(t, def, curr, action, shared, t.StepCount+1, execErr)
}

func (e *Engine) execExecutor(node DefNode, input interface{}, params map[string]interface{}) (interface{}, string, string, error) {
	lst, _ := e.Store.ListWorkers(node.Service, 15)
	if len(lst) == 0 {
		return nil, "", "", errorString("no worker")
	}
	if node.WeightedByLoad {
		sort.SliceStable(lst, func(i, j int) bool { return lst[i].Load < lst[j].Load })
	}
	payload := map[string]interface{}{"input": input, "params": params}
	b, _ := json.Marshal(payload)
	attempts := 0
	for _, w := range lst {
		attempts++
		endpoint := w.URL + "/exec/" + node.Service
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(b))
		if err != nil {
			cancel()
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := e.HTTP.Do(req)
		if err != nil {
			cancel()
			continue
		}
		var out struct {
			Result interface{} `json:"result"`
			Error  string      `json:"error"`
		}
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(&out); err != nil {
			resp.Body.Close()
			cancel()
			continue
		}
		resp.Body.Close()
		cancel()
		if out.Error != "" {
			if node.AttemptDelayMillis > 0 {
				time.Sleep(time.Duration(node.AttemptDelayMillis) * time.Millisecond)
			}
			if node.MaxAttempts > 0 && attempts >= node.MaxAttempts {
				break
			}
			continue
		}
		return out.Result, w.ID, w.URL, nil
	}
	return nil, "", "", errorString("all workers failed")
}
