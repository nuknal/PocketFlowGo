package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

func readDefs(dir string) ([]string, []string, error) {
	entries := []fs.DirEntry{}
	d, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range d {
		if !e.Type().IsRegular() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".json") {
			entries = append(entries, e)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	names := make([]string, 0, len(entries))
	contents := make([]string, 0, len(entries))
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, nil, err
		}
		names = append(names, strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
		contents = append(contents, string(b))
	}
	return names, contents, nil
}

func deriveParams(def map[string]interface{}) map[string]interface{} {
	params := map[string]interface{}{}
	nodes, _ := def["nodes"].(map[string]interface{})
	for _, nv := range nodes {
		nd, _ := nv.(map[string]interface{})
		kind, _ := nd["kind"].(string)
		if kind == "approval" {
			pm, _ := nd["params"].(map[string]interface{})
			if pm != nil {
				if ak, ok := pm["approval_key"].(string); ok && strings.HasPrefix(ak, "$params.") {
					k := strings.TrimPrefix(ak, "$params.")
					if k != "" && k[0] == '.' {
						k = k[1:]
					}
					post, _ := nd["post"].(map[string]interface{})
					if post != nil {
						if _, ok := post["action_key"].(string); ok {
							params[k] = "approved"
						} else {
							params[k] = true
						}
					} else {
						params[k] = true
					}
				}
			}
		}
		prep, _ := nd["prep"].(map[string]interface{})
		if prep != nil {
			if ik, ok := prep["input_key"].(string); ok {
				if strings.HasPrefix(ik, "$params.") {
					k := strings.TrimPrefix(ik, "$params.")
					if k != "" && k[0] == '.' {
						k = k[1:]
					}
					switch k {
					case "val":
						params["val"] = 2.0
					case "text":
						params["text"] = "Hello PocketFlow"
					case "list":
						params["list"] = []interface{}{1.0, 2.0, 3.0}
					default:
						params[k] = 1.0
					}
				}
			}
		}
		if kind == "choice" {
			cases, _ := nd["choice_cases"].([]interface{})
			if len(cases) > 0 {
				params["path"] = pickFirstAction(cases)
			}
			if params["path"] == nil {
				params["path"] = "B"
			}
		}
	}
	return params
}

func pickFirstAction(cases []interface{}) string {
	for _, c := range cases {
		m, _ := c.(map[string]interface{})
		if act, ok := m["action"].(string); ok && act != "" {
			return act
		}
	}
	return "B"
}

func detectSignals(def map[string]interface{}) map[string]interface{} {
	sigs := map[string]interface{}{}
	nodes, _ := def["nodes"].(map[string]interface{})
	for _, nv := range nodes {
		nd, _ := nv.(map[string]interface{})
		kind, _ := nd["kind"].(string)
		if kind == "wait_event" {
			pm, _ := nd["params"].(map[string]interface{})
			if s, ok := pm["signal_key"].(string); ok && strings.HasPrefix(s, "$shared.") {
				k := strings.TrimPrefix(s, "$shared.")
				if k != "" && k[0] == '.' {
					k = k[1:]
				}
				if k == "flag" {
					sigs[k] = true
				} else {
					sigs[k] = true
				}
			}
		}
		if kind == "approval" {
			pm, _ := nd["params"].(map[string]interface{})
			if s, ok := pm["approval_key"].(string); ok && strings.HasPrefix(s, "$shared.") {
				k := strings.TrimPrefix(s, "$shared.")
				if k != "" && k[0] == '.' {
					k = k[1:]
				}
				post, _ := nd["post"].(map[string]interface{})
				if post != nil {
					if _, ok := post["action_key"].(string); ok {
						sigs[k] = "approved"
					} else {
						sigs[k] = true
					}
				} else {
					sigs[k] = true
				}
			}
		}
		prep, _ := nd["prep"].(map[string]interface{})
		if prep != nil {
			if ik, ok := prep["input_key"].(string); ok && strings.HasPrefix(ik, "$shared.") {
				k := strings.TrimPrefix(ik, "$shared.")
				if k != "" && k[0] == '.' {
					k = k[1:]
				}
				if k == "list" {
					sigs[k] = []interface{}{1.0, 2.0, 3.0}
				}
			}
		}
	}
	return sigs
}

func main() {
	base := os.Getenv("SCHEDULER_BASE")
	if base == "" {
		base = "http://localhost:8070/api"
	}
	dir := "examples/flowjson"
	names, contents, err := readDefs(dir)
	if err != nil {
		fmt.Println("ERR", err)
		return
	}
	for i := range names {
		name := names[i]
		defStr := contents[i]
		var def map[string]interface{}
		_ = json.Unmarshal([]byte(defStr), &def)
		var flowResp map[string]string
		_ = postJSON(base, "/flows", map[string]string{"Name": name}, &flowResp)
		flowID := flowResp["id"]
		fmt.Println("FLOW", name)
		fmt.Println(defStr)
		var verResp map[string]string
		_ = postJSON(base, "/flows/version", map[string]interface{}{"FlowID": flowID, "Version": 1, "DefinitionJSON": defStr, "Status": "published"}, &verResp)
		params := deriveParams(def)
		paramsStr, _ := json.Marshal(params)
		var tResp map[string]string
		_ = postJSON(base, "/tasks", map[string]interface{}{"FlowID": flowID, "ParamsJSON": string(paramsStr)}, &tResp)
		taskID := tResp["id"]
		sigs := detectSignals(def)
		if len(sigs) > 0 {
			time.Sleep(300 * time.Millisecond)
			for k, v := range sigs {
				_ = postJSON(base, "/tasks/signal", map[string]interface{}{"task_id": taskID, "key": k, "value": v}, nil)
			}
		}
		for j := 0; j < 40; j++ {
			var gt struct {
				ID         string
				Status     string
				SharedJSON string
			}
			_ = getJSON(base, "/tasks/get?id="+taskID, &gt)
			fmt.Println("TASK", taskID, "STATUS", gt.Status)
			if gt.Status == "completed" || gt.Status == "canceled" || gt.Status == "failed" {
				break
			}
			time.Sleep(300 * time.Millisecond)
		}
		var gt2 struct {
			ID         string
			Status     string
			SharedJSON string
		}
		_ = getJSON(base, "/tasks/get?id="+taskID, &gt2)
		var shared map[string]interface{}
		_ = json.Unmarshal([]byte(gt2.SharedJSON), &shared)
		fmt.Println("TASK", taskID, "SHARED", shared)
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
