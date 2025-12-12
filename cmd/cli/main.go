package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

func postJSON(base string, path string, payload interface{}, out interface{}) error {
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, base+path, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func getJSON(base string, path string, out interface{}) error {
	req, _ := http.NewRequest(http.MethodGet, base+path, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func main() {
	base := os.Getenv("SCHEDULER_BASE")
	if base == "" {
		base = "http://localhost:8070/api"
	}
	examples := []struct {
		name   string
		def    map[string]interface{}
		params []map[string]interface{}
	}{
		{
			name: "branch-demo",
			def: map[string]interface{}{
				"start": "decide",
				"nodes": map[string]interface{}{
					"decide": map[string]interface{}{"kind": "executor", "service": "route", "params": map[string]interface{}{}, "post": map[string]interface{}{"action_key": "action"}},
					"B":      map[string]interface{}{"kind": "executor", "service": "transform", "params": map[string]interface{}{"op": "upper"}, "prep": map[string]interface{}{"input_key": "$params.text"}, "post": map[string]interface{}{"output_key": "result", "action_static": "default"}},
					"C":      map[string]interface{}{"kind": "executor", "service": "transform", "params": map[string]interface{}{"op": "lower"}, "prep": map[string]interface{}{"input_key": "$params.text"}, "post": map[string]interface{}{"output_key": "result", "action_static": "default"}},
				},
				"edges": []map[string]interface{}{
					{"from": "decide", "action": "goB", "to": "B"},
					{"from": "decide", "action": "goC", "to": "C"},
					{"from": "B", "action": "default", "to": ""},
					{"from": "C", "action": "default", "to": ""},
				},
			},
			params: []map[string]interface{}{
				{"text": "Hello PocketFlow", "action": "goB"},
				{"text": "Hello PocketFlow", "action": "goC"},
			},
		},
		{
			name: "parallel-agg",
			def: map[string]interface{}{
				"start": "p",
				"nodes": map[string]interface{}{
					"p": map[string]interface{}{
						"kind":              "parallel",
						"parallel_services": []string{"transform", "route"},
						"prep":              map[string]interface{}{"input_key": "$params.val"},
						"params":            map[string]interface{}{"mul": 3.0, "action": "goX"},
						"post":              map[string]interface{}{"output_key": "agg", "action_static": "next"},
						"parallel_mode":     "concurrent",
						"max_parallel":      2,
					},
					"end": map[string]interface{}{
						"kind":    "executor",
						"service": "transform",
						"prep":    map[string]interface{}{"input_key": "$params.val"},
						"params":  map[string]interface{}{"mul": 1.0},
						"post":    map[string]interface{}{"action_static": ""},
					},
				},
				"edges": []map[string]interface{}{{"from": "p", "action": "next", "to": "end"}},
			},
			params: []map[string]interface{}{{"val": 2.0}},
		},
		{
			name: "subflow-demo",
			def: func() map[string]interface{} {
				sub := map[string]interface{}{
					"start": "a",
					"nodes": map[string]interface{}{
						"a": map[string]interface{}{"kind": "executor", "service": "transform", "prep": map[string]interface{}{"input_key": "$params.val"}, "params": map[string]interface{}{"mul": 50.0}, "post": map[string]interface{}{"output_key": "m", "action_static": "go"}},
						"b": map[string]interface{}{"kind": "executor", "service": "route", "prep": map[string]interface{}{"input_key": "m"}, "params": map[string]interface{}{"action": "goC"}, "post": map[string]interface{}{"action_static": "done"}},
					},
					"edges": []map[string]interface{}{{"from": "a", "action": "go", "to": "b"}},
				}
				return map[string]interface{}{
					"start": "sf",
					"nodes": map[string]interface{}{
						"sf":  map[string]interface{}{"kind": "subflow", "subflow": sub, "post": map[string]interface{}{"output_key": "sub_out", "action_static": "next"}},
						"end": map[string]interface{}{"kind": "executor", "service": "transform", "prep": map[string]interface{}{"input_key": "$shared.sub_out.m"}, "params": map[string]interface{}{"mul": 100.0}, "post": map[string]interface{}{"output_key": "end_out", "action_static": ""}},
					},
					"edges": []map[string]interface{}{{"from": "sf", "action": "next", "to": "end"}},
				}
			}(),
			params: []map[string]interface{}{{"val": 20.0}},
		},
		{
			name: "parallel-failfast",
			def: map[string]interface{}{
				"start": "p",
				"nodes": map[string]interface{}{
					"p": map[string]interface{}{
						"kind":              "parallel",
						"parallel_services": []string{"transform", "bad"},
						"parallel_mode":     "concurrent",
						"failure_strategy":  "fail_fast",
						"prep":              map[string]interface{}{"input_key": "$params.val"},
						"params":            map[string]interface{}{"mul": 2.0},
						"post":              map[string]interface{}{"output_key": "agg", "action_static": "next"},
					},
					"end": map[string]interface{}{
						"kind":    "executor",
						"service": "transform",
						"prep":    map[string]interface{}{"input_key": "$params.val"},
						"params":  map[string]interface{}{"mul": 1.0},
						"post":    map[string]interface{}{"action_static": ""},
					},
				},
				"edges": []map[string]interface{}{{"from": "p", "action": "next", "to": "end"}},
			},
			params: []map[string]interface{}{{"val": 2.0}},
		},
		{
			name: "full-demo",
			def: func() map[string]interface{} {
				sub := map[string]interface{}{
					"start": "a",
					"nodes": map[string]interface{}{
						"a": map[string]interface{}{"kind": "executor", "service": "transform", "prep": map[string]interface{}{"input_key": "$params.val"}, "params": map[string]interface{}{"mul": 5.0}, "post": map[string]interface{}{"output_key": "m", "action_static": "go"}},
						"b": map[string]interface{}{"kind": "executor", "service": "route", "prep": map[string]interface{}{"input_key": "m"}, "params": map[string]interface{}{"action": "goC"}, "post": map[string]interface{}{"action_static": "done"}},
					},
					"edges": []map[string]interface{}{{"from": "a", "action": "go", "to": "b"}},
				}
				return map[string]interface{}{
					"start": "choice",
					"nodes": map[string]interface{}{
						"choice":   map[string]interface{}{"kind": "choice", "choice_cases": []map[string]interface{}{{"action": "approval", "expr": map[string]interface{}{"eq": []interface{}{"$params.path", "approval"}}}}, "default_action": "parallel"},
						"approval": map[string]interface{}{"kind": "approval", "params": map[string]interface{}{"approval_key": "$shared.approval"}, "post": map[string]interface{}{"action_key": "approval"}},
						"timer":    map[string]interface{}{"kind": "timer", "params": map[string]interface{}{"delay_ms": 100}, "post": map[string]interface{}{"action_static": "go"}},
						"foreach":  map[string]interface{}{"kind": "foreach", "service": "transform", "prep": map[string]interface{}{"input_key": "$shared.list"}, "params": map[string]interface{}{"mul": 2.0}, "post": map[string]interface{}{"output_key": "mapped", "action_static": "go"}, "parallel_mode": "sequential"},
						"parallel": map[string]interface{}{"kind": "parallel", "parallel_services": []string{"transform", "transform"}, "prep": map[string]interface{}{"input_key": "$params.val"}, "params": map[string]interface{}{"mul": 3.0}, "post": map[string]interface{}{"output_key": "agg", "action_static": "next"}, "parallel_mode": "concurrent", "max_parallel": 2},
						"wait":     map[string]interface{}{"kind": "wait_event", "params": map[string]interface{}{"signal_key": "$shared.flag", "timeout_ms": 1000}, "post": map[string]interface{}{"action_static": "go"}},
						"sf":       map[string]interface{}{"kind": "subflow", "subflow": sub, "post": map[string]interface{}{"output_key": "sub_out", "action_static": "next"}},
						"end":      map[string]interface{}{"kind": "executor", "service": "transform", "prep": map[string]interface{}{"input_key": "$shared.sub_out.m"}, "params": map[string]interface{}{"mul": 100.0}, "post": map[string]interface{}{"output_key": "end_out", "action_static": ""}},
					},
					"edges": []map[string]interface{}{
						{"from": "choice", "action": "approval", "to": "approval"},
						{"from": "choice", "action": "parallel", "to": "parallel"},
						{"from": "approval", "action": "approved", "to": "timer"},
						{"from": "approval", "action": "rejected", "to": "parallel"},
						{"from": "timer", "action": "go", "to": "foreach"},
						{"from": "foreach", "action": "go", "to": "wait"},
						{"from": "parallel", "action": "next", "to": "wait"},
						{"from": "wait", "action": "go", "to": "sf"},
						{"from": "sf", "action": "next", "to": "end"},
					},
				}
			}(),
			params: []map[string]interface{}{{"val": 1234.0, "path": "approval"}},
		},
	}
	for _, ex := range examples {
		var flowResp map[string]string
		_ = postJSON(base, "/flows", map[string]string{"Name": ex.name}, &flowResp)
		flowID := flowResp["id"]
		defJSONBytes, _ := json.MarshalIndent(ex.def, "", "  ")
		fmt.Println("FLOW", ex.name)
		fmt.Println(string(defJSONBytes))
		var verResp map[string]string
		_ = postJSON(base, "/flows/version", map[string]interface{}{"FlowID": flowID, "Version": 1, "DefinitionJSON": string(defJSONBytes), "Status": "published"}, &verResp)
		for _, p := range ex.params {
			paramsStr, _ := json.Marshal(p)
			var tResp map[string]string
			_ = postJSON(base, "/tasks", map[string]interface{}{"FlowID": flowID, "ParamsJSON": string(paramsStr)}, &tResp)
			taskID := tResp["id"]
			for i := 0; i < 30; i++ {
				var gt struct {
					ID         string
					Status     string
					SharedJSON string
				}
				_ = getJSON(base, "/tasks/get?id="+taskID, &gt)
				fmt.Println("TASK", taskID, "STATUS", gt.Status)
				if ex.name == "full-demo" && i == 2 {
					_ = postJSON(base, "/tasks/signal", map[string]interface{}{"task_id": taskID, "key": "approval", "value": "approved"}, nil)
					_ = postJSON(base, "/tasks/signal", map[string]interface{}{"task_id": taskID, "key": "flag", "value": true}, nil)
					_ = postJSON(base, "/tasks/signal", map[string]interface{}{"task_id": taskID, "key": "list", "value": []interface{}{1.0, 2.0, 3.0}}, nil)
				}
				if gt.Status == "completed" || gt.Status == "canceled" || gt.Status == "failed" {
					break
				}
				time.Sleep(300 * time.Millisecond)
			}
			var gt struct {
				ID         string
				Status     string
				SharedJSON string
			}
			_ = getJSON(base, "/tasks/get?id="+taskID, &gt)
			var shared map[string]interface{}
			_ = json.Unmarshal([]byte(gt.SharedJSON), &shared)
			fmt.Println("TASK", taskID, "SHARED", shared)
			final := shared["end_out"]
			if final == nil {
				final = shared["result"]
			}
			if final == nil {
				final = shared["agg"]
			}
			if final == nil {
				final = shared["sub_out"]
			}
			if final != nil {
				fmt.Println("FINAL", final)
			}
			var runs []map[string]interface{}
			_ = getJSON(base, "/tasks/runs?task_id="+taskID, &runs)
			for _, r := range runs {
				nk := r["nodeKey"]
				if nk == nil {
					nk = r["NodeKey"]
				}
				at := r["attemptNo"]
				if at == nil {
					at = r["AttemptNo"]
				}
				st := r["status"]
				if st == nil {
					st = r["Status"]
				}
				ac := r["action"]
				if ac == nil {
					ac = r["Action"]
				}
				wid := r["workerId"]
				if wid == nil {
					wid = r["WorkerId"]
				}
				wurl := r["workerUrl"]
				if wurl == nil {
					wurl = r["WorkerUrl"]
				}
				fmt.Println("RUN", nk, at, st, ac, wid, wurl)
			}
		}
	}
}
