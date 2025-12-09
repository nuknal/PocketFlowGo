package engine

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nuknal/PocketFlowGo/internal/store"
)

type execRequest struct {
	Input  interface{}            `json:"input"`
	Params map[string]interface{} `json:"params"`
}
type execResponse struct {
	Result interface{} `json:"result"`
	Error  string      `json:"error,omitempty"`
}

func startWorker(t *testing.T, s *store.SQLite) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/exec/transform", func(w http.ResponseWriter, r *http.Request) {
		var req execRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch v := req.Input.(type) {
		case string:
			op, _ := req.Params["op"].(string)
			if op == "upper" {
				_ = json.NewEncoder(w).Encode(execResponse{Result: bytes.ToUpper([]byte(v))})
				return
			}
			if op == "lower" {
				_ = json.NewEncoder(w).Encode(execResponse{Result: bytes.ToLower([]byte(v))})
				return
			}
			_ = json.NewEncoder(w).Encode(execResponse{Error: "unsupported"})
			return
		case float64:
			mul := 1.0
			if m, ok := req.Params["mul"].(float64); ok {
				mul = m
			}
			_ = json.NewEncoder(w).Encode(execResponse{Result: v * mul})
			return
		default:
			_ = json.NewEncoder(w).Encode(execResponse{Error: "bad input"})
			return
		}
	})
	mux.HandleFunc("/exec/sum", func(w http.ResponseWriter, r *http.Request) {
		var req execRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch arr := req.Input.(type) {
		case []interface{}:
			s := 0.0
			for _, x := range arr {
				if f, ok := x.(float64); ok {
					s += f
				}
			}
			_ = json.NewEncoder(w).Encode(execResponse{Result: s})
			return
		default:
			_ = json.NewEncoder(w).Encode(execResponse{Error: "bad input"})
			return
		}
	})
	mux.HandleFunc("/exec/route", func(w http.ResponseWriter, r *http.Request) {
		var req execRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		act := "goB"
		if v, ok := req.Params["action"].(string); ok && v != "" {
			act = v
		}
		_ = json.NewEncoder(w).Encode(execResponse{Result: map[string]interface{}{"action": act}})
	})
	srv := httptest.NewServer(mux)
	wi := store.WorkerInfo{ID: "w1", URL: srv.URL, Services: []string{"transform", "sum", "route"}, Load: 0, LastHeartbeat: time.Now().Unix(), Status: "online"}
	_ = s.RegisterWorker(wi)
	return srv
}

func openTestStore(t *testing.T) *store.SQLite {
	p := t.TempDir() + "/test.db"
	s, err := store.OpenSQLite(p)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return s
}

func startBadWorker(t *testing.T, s *store.SQLite) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/exec/transform", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(execResponse{Error: "fail"})
	})
	srv := httptest.NewServer(mux)
	wi := store.WorkerInfo{ID: "w2", URL: srv.URL, Services: []string{"transform", "sum", "route", "bad"}, Load: 10, LastHeartbeat: time.Now().Unix(), Status: "online"}
	_ = s.RegisterWorker(wi)
	return srv
}

func TestChoiceNode(t *testing.T) {
	s := openTestStore(t)
	srv := startWorker(t, s)
	defer srv.Close()
	fid, err := s.CreateFlow("f1")
	if err != nil {
		t.Fatalf("%v", err)
	}
	def := map[string]interface{}{
		"start": "choice",
		"nodes": map[string]interface{}{
			"choice": map[string]interface{}{
				"kind":           "choice",
				"post":           map[string]interface{}{"action_key": "flag"},
				"default_action": "transform",
			},
			"transform": map[string]interface{}{
				"kind":    "executor",
				"service": "transform",
				"prep":    map[string]interface{}{"input_key": "$params.val"},
				"params":  map[string]interface{}{"mul": 2.0},
				"post":    map[string]interface{}{"output_key": "out1", "action_static": "done"},
			},
			"sum": map[string]interface{}{
				"kind":    "executor",
				"service": "sum",
				"prep":    map[string]interface{}{"input_key": "arr"},
				"post":    map[string]interface{}{"output_key": "out2", "action_static": "done"},
			},
		},
		"edges": []map[string]interface{}{
			{"from": "choice", "action": "transform", "to": "transform"},
			{"from": "choice", "action": "sum", "to": "sum"},
		},
	}
	b, _ := json.Marshal(def)
	vid, err := s.CreateFlowVersion(fid, 1, string(b), "published")
	if err != nil {
		t.Fatalf("%v", err)
	}
	p := map[string]interface{}{"val": 3.0}
	pb, _ := json.Marshal(p)
	tid, err := s.CreateTask(vid, string(pb), "", "choice")
	if err != nil {
		t.Fatalf("%v", err)
	}
	tsk, _ := s.GetTask(tid)
	sh := map[string]interface{}{"flag": "transform"}
	shb, _ := json.Marshal(sh)
	_ = s.UpdateTaskProgress(tsk.ID, tsk.CurrentNodeKey, "", string(shb), tsk.StepCount)
	e := New(s)
	for i := 0; i < 10; i++ {
		_ = e.RunOnce(tid)
		nt, _ := s.GetTask(tid)
		if nt.Status == "completed" || nt.CurrentNodeKey == "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("not completed")
}

func TestParallelNode(t *testing.T) {
	s := openTestStore(t)
	srv := startWorker(t, s)
	defer srv.Close()
	fid, err := s.CreateFlow("f2")
	if err != nil {
		t.Fatalf("%v", err)
	}
	def := map[string]interface{}{
		"start": "p",
		"nodes": map[string]interface{}{
			"p": map[string]interface{}{
				"kind":              "parallel",
				"parallel_services": []string{"transform", "route"},
				"prep":              map[string]interface{}{"input_key": "$params.val"},
				"params":            map[string]interface{}{"mul": 3.0, "action": "goX"},
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
		"edges": []map[string]interface{}{
			{"from": "p", "action": "next", "to": "end"},
		},
	}
	b, _ := json.Marshal(def)
	vid, err := s.CreateFlowVersion(fid, 1, string(b), "published")
	if err != nil {
		t.Fatalf("%v", err)
	}
	p := map[string]interface{}{"val": 2.0}
	pb, _ := json.Marshal(p)
	tid, err := s.CreateTask(vid, string(pb), "", "p")
	if err != nil {
		t.Fatalf("%v", err)
	}
	e := New(s)
	for i := 0; i < 50; i++ {
		_ = e.RunOnce(tid)
		nt, _ := s.GetTask(tid)
		if nt.Status == "completed" || nt.CurrentNodeKey == "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("not completed")
}

func TestSubflowNode(t *testing.T) {
	s := openTestStore(t)
	srv := startWorker(t, s)
	defer srv.Close()
	fid, err := s.CreateFlow("f3")
	if err != nil {
		t.Fatalf("%v", err)
	}
	sub := map[string]interface{}{
		"start": "a",
		"nodes": map[string]interface{}{
			"a": map[string]interface{}{
				"kind":    "executor",
				"service": "transform",
				"prep":    map[string]interface{}{"input_key": "$params.val"},
				"params":  map[string]interface{}{"mul": 5.0},
				"post":    map[string]interface{}{"output_key": "m", "action_static": "go"},
			},
			"b": map[string]interface{}{
				"kind":    "executor",
				"service": "route",
				"prep":    map[string]interface{}{"input_key": "m"},
				"params":  map[string]interface{}{"action": "goC"},
				"post":    map[string]interface{}{"action_static": "done"},
			},
		},
		"edges": []map[string]interface{}{
			{"from": "a", "action": "go", "to": "b"},
		},
	}
	def := map[string]interface{}{
		"start": "sf",
		"nodes": map[string]interface{}{
			"sf": map[string]interface{}{
				"kind":    "subflow",
				"subflow": sub,
				"post":    map[string]interface{}{"output_key": "sub_out", "action_static": "next"},
			},
			"end": map[string]interface{}{
				"kind":    "executor",
				"service": "transform",
				"prep":    map[string]interface{}{"input_key": "$params.val"},
				"params":  map[string]interface{}{"mul": 1.0},
				"post":    map[string]interface{}{"action_static": ""},
			},
		},
		"edges": []map[string]interface{}{
			{"from": "sf", "action": "next", "to": "end"},
		},
	}
	b, _ := json.Marshal(def)
	vid, err := s.CreateFlowVersion(fid, 1, string(b), "published")
	if err != nil {
		t.Fatalf("%v", err)
	}
	p := map[string]interface{}{"val": 2.0}
	pb, _ := json.Marshal(p)
	tid, err := s.CreateTask(vid, string(pb), "", "sf")
	if err != nil {
		t.Fatalf("%v", err)
	}
	e := New(s)
	for i := 0; i < 50; i++ {
		_ = e.RunOnce(tid)
		nt, _ := s.GetTask(tid)
		if nt.Status == "completed" || nt.CurrentNodeKey == "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("not completed")
}

func TestExecutorWeightedByLoad(t *testing.T) {
	s := openTestStore(t)
	srv := startWorker(t, s)
	srvBad := startBadWorker(t, s)
	defer srv.Close()
	defer srvBad.Close()
	fid, err := s.CreateFlow("f4")
	if err != nil {
		t.Fatalf("%v", err)
	}
	def := map[string]interface{}{
		"start": "x",
		"nodes": map[string]interface{}{
			"x": map[string]interface{}{
				"kind":             "executor",
				"service":          "transform",
				"prep":             map[string]interface{}{"input_key": "$params.val"},
				"params":           map[string]interface{}{"mul": 2.0},
				"post":             map[string]interface{}{"output_key": "out", "action_static": ""},
				"weighted_by_load": true,
			},
		},
		"edges": []map[string]interface{}{},
	}
	b, _ := json.Marshal(def)
	vid, err := s.CreateFlowVersion(fid, 1, string(b), "published")
	if err != nil {
		t.Fatalf("%v", err)
	}
	p := map[string]interface{}{"val": 3.0}
	pb, _ := json.Marshal(p)
	tid, err := s.CreateTask(vid, string(pb), "", "x")
	if err != nil {
		t.Fatalf("%v", err)
	}
	e := New(s)
	for i := 0; i < 20; i++ {
		_ = e.RunOnce(tid)
		nt, _ := s.GetTask(tid)
		if nt.Status == "completed" || nt.CurrentNodeKey == "" {
			runs, _ := s.ListNodeRuns(tid)
			if len(runs) == 0 {
				t.Fatalf("no runs")
			}
			if runs[0].WorkerURL != srv.URL {
				t.Fatalf("wrong worker")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("not completed")
}

func TestCancelingTask(t *testing.T) {
	s := openTestStore(t)
	srv := startWorker(t, s)
	defer srv.Close()
	fid, err := s.CreateFlow("f5")
	if err != nil {
		t.Fatalf("%v", err)
	}
	def := map[string]interface{}{
		"start": "x",
		"nodes": map[string]interface{}{
			"x": map[string]interface{}{
				"kind":    "executor",
				"service": "transform",
				"prep":    map[string]interface{}{"input_key": "$params.val"},
				"params":  map[string]interface{}{"mul": 2.0},
				"post":    map[string]interface{}{"output_key": "out", "action_static": ""},
			},
		},
		"edges": []map[string]interface{}{},
	}
	b, _ := json.Marshal(def)
	vid, err := s.CreateFlowVersion(fid, 1, string(b), "published")
	if err != nil {
		t.Fatalf("%v", err)
	}
	p := map[string]interface{}{"val": 3.0}
	pb, _ := json.Marshal(p)
	tid, err := s.CreateTask(vid, string(pb), "", "x")
	if err != nil {
		t.Fatalf("%v", err)
	}
	_ = s.UpdateTaskStatus(tid, "canceling")
	e := New(s)
	_ = e.RunOnce(tid)
	nt, _ := s.GetTask(tid)
	if nt.Status != "canceled" {
		t.Fatalf("not canceled")
	}
	runs, _ := s.ListNodeRuns(tid)
	if len(runs) == 0 || runs[0].Status != "canceled" {
		t.Fatalf("no canceled run")
	}
}

func TestChoiceExprComplex(t *testing.T) {
	s := openTestStore(t)
	srv := startWorker(t, s)
	defer srv.Close()
	fid, err := s.CreateFlow("f6")
	if err != nil {
		t.Fatalf("%v", err)
	}
	def := map[string]interface{}{
		"start": "choice",
		"nodes": map[string]interface{}{
			"choice": map[string]interface{}{
				"kind": "choice",
				"choice_cases": []map[string]interface{}{
					{"action": "sum", "expr": map[string]interface{}{"and": []interface{}{map[string]interface{}{"eq": []interface{}{"$params.flag", "sum"}}, map[string]interface{}{"gt": []interface{}{"$params.val", 1}}}}},
				},
				"default_action": "transform",
			},
			"transform": map[string]interface{}{
				"kind":    "executor",
				"service": "transform",
				"prep":    map[string]interface{}{"input_key": "$params.val"},
				"params":  map[string]interface{}{"mul": 2.0},
				"post":    map[string]interface{}{"output_key": "out1", "action_static": "done"},
			},
			"sum": map[string]interface{}{
				"kind":    "executor",
				"service": "sum",
				"prep":    map[string]interface{}{"input_key": "arr"},
				"post":    map[string]interface{}{"output_key": "out2", "action_static": "done"},
			},
		},
		"edges": []map[string]interface{}{
			{"from": "choice", "action": "transform", "to": "transform"},
			{"from": "choice", "action": "sum", "to": "sum"},
		},
	}
	b, _ := json.Marshal(def)
	vid, err := s.CreateFlowVersion(fid, 1, string(b), "published")
	if err != nil {
		t.Fatalf("%v", err)
	}
	p := map[string]interface{}{"val": 3.0, "flag": "sum"}
	pb, _ := json.Marshal(p)
	tid, err := s.CreateTask(vid, string(pb), "", "choice")
	if err != nil {
		t.Fatalf("%v", err)
	}
	tsk, _ := s.GetTask(tid)
	sh := map[string]interface{}{"arr": []interface{}{1.0, 2.0, 3.0}}
	shb, _ := json.Marshal(sh)
	_ = s.UpdateTaskProgress(tsk.ID, tsk.CurrentNodeKey, "", string(shb), tsk.StepCount)
	e := New(s)
	for i := 0; i < 20; i++ {
		_ = e.RunOnce(tid)
		nt, _ := s.GetTask(tid)
		if nt.Status == "completed" || nt.CurrentNodeKey == "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("not completed")
}

func TestParallelFailFast(t *testing.T) {
	s := openTestStore(t)
	srv := startWorker(t, s)
	srvBad := startBadWorker(t, s)
	defer srv.Close()
	defer srvBad.Close()
	fid, err := s.CreateFlow("f7")
	if err != nil {
		t.Fatalf("%v", err)
	}
	def := map[string]interface{}{
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
		"edges": []map[string]interface{}{
			{"from": "p", "action": "next", "to": "end"},
		},
	}
	b, _ := json.Marshal(def)
	vid, err := s.CreateFlowVersion(fid, 1, string(b), "published")
	if err != nil {
		t.Fatalf("%v", err)
	}
	p := map[string]interface{}{"val": 2.0}
	pb, _ := json.Marshal(p)
	tid, err := s.CreateTask(vid, string(pb), "", "p")
	if err != nil {
		t.Fatalf("%v", err)
	}
	e := New(s)
	for i := 0; i < 50; i++ {
		_ = e.RunOnce(tid)
		nt, _ := s.GetTask(tid)
		if nt.Status == "completed" || nt.CurrentNodeKey == "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("not completed")
}

func TestTimerNode(t *testing.T) {
	s := openTestStore(t)
	srv := startWorker(t, s)
	defer srv.Close()
	fid, err := s.CreateFlow("f8")
	if err != nil {
		t.Fatalf("%v", err)
	}
	def := map[string]interface{}{
		"start": "tm",
		"nodes": map[string]interface{}{
			"tm":  map[string]interface{}{"kind": "timer", "params": map[string]interface{}{"delay_ms": 50}, "post": map[string]interface{}{"action_static": "go"}},
			"end": map[string]interface{}{"kind": "executor", "service": "transform", "prep": map[string]interface{}{"input_key": "$params.val"}, "params": map[string]interface{}{"mul": 1.0}, "post": map[string]interface{}{"action_static": ""}},
		},
		"edges": []map[string]interface{}{{"from": "tm", "action": "go", "to": "end"}},
	}
	b, _ := json.Marshal(def)
	vid, err := s.CreateFlowVersion(fid, 1, string(b), "published")
	if err != nil {
		t.Fatalf("%v", err)
	}
	p := map[string]interface{}{"val": 2.0}
	pb, _ := json.Marshal(p)
	tid, err := s.CreateTask(vid, string(pb), "", "tm")
	if err != nil {
		t.Fatalf("%v", err)
	}
	e := New(s)
	for i := 0; i < 50; i++ {
		_ = e.RunOnce(tid)
		nt, _ := s.GetTask(tid)
		if nt.Status == "completed" || nt.CurrentNodeKey == "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("not completed")
}

func TestForeachNode(t *testing.T) {
	s := openTestStore(t)
	srv := startWorker(t, s)
	defer srv.Close()
	fid, err := s.CreateFlow("f9")
	if err != nil {
		t.Fatalf("%v", err)
	}
	def := map[string]interface{}{
		"start": "fe",
		"nodes": map[string]interface{}{
			"fe":  map[string]interface{}{"kind": "foreach", "service": "transform", "prep": map[string]interface{}{"input_key": "$shared.arr"}, "params": map[string]interface{}{"mul": 2.0}, "post": map[string]interface{}{"output_key": "mapped", "action_static": "go"}, "parallel_mode": "sequential"},
			"end": map[string]interface{}{"kind": "executor", "service": "transform", "prep": map[string]interface{}{"input_key": "$params.val"}, "params": map[string]interface{}{"mul": 1.0}, "post": map[string]interface{}{"action_static": ""}},
		},
		"edges": []map[string]interface{}{{"from": "fe", "action": "go", "to": "end"}},
	}
	b, _ := json.Marshal(def)
	vid, err := s.CreateFlowVersion(fid, 1, string(b), "published")
	if err != nil {
		t.Fatalf("%v", err)
	}
	p := map[string]interface{}{"val": 2.0}
	pb, _ := json.Marshal(p)
	tid, err := s.CreateTask(vid, string(pb), "", "fe")
	if err != nil {
		t.Fatalf("%v", err)
	}
	tsk, _ := s.GetTask(tid)
	sh := map[string]interface{}{"arr": []interface{}{1.0, 2.0, 3.0}}
	shb, _ := json.Marshal(sh)
	_ = s.UpdateTaskProgress(tsk.ID, tsk.CurrentNodeKey, "", string(shb), tsk.StepCount)
	e := New(s)
	for i := 0; i < 100; i++ {
		_ = e.RunOnce(tid)
		nt, _ := s.GetTask(tid)
		if nt.Status == "completed" || nt.CurrentNodeKey == "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("not completed")
}

func TestWaitEventNode(t *testing.T) {
	s := openTestStore(t)
	srv := startWorker(t, s)
	defer srv.Close()
	fid, err := s.CreateFlow("f10")
	if err != nil {
		t.Fatalf("%v", err)
	}
	def := map[string]interface{}{
		"start": "we",
		"nodes": map[string]interface{}{
			"we":  map[string]interface{}{"kind": "wait_event", "params": map[string]interface{}{"signal_key": "$shared.flag", "timeout_ms": 500}, "post": map[string]interface{}{"action_static": "go"}},
			"end": map[string]interface{}{"kind": "executor", "service": "transform", "prep": map[string]interface{}{"input_key": "$params.val"}, "params": map[string]interface{}{"mul": 1.0}, "post": map[string]interface{}{"action_static": ""}},
		},
		"edges": []map[string]interface{}{{"from": "we", "action": "go", "to": "end"}},
	}
	b, _ := json.Marshal(def)
	vid, err := s.CreateFlowVersion(fid, 1, string(b), "published")
	if err != nil {
		t.Fatalf("%v", err)
	}
	p := map[string]interface{}{"val": 2.0}
	pb, _ := json.Marshal(p)
	tid, err := s.CreateTask(vid, string(pb), "", "we")
	if err != nil {
		t.Fatalf("%v", err)
	}
	e := New(s)
	for i := 0; i < 5; i++ {
		_ = e.RunOnce(tid)
		time.Sleep(10 * time.Millisecond)
	}
	nt, _ := s.GetTask(tid)
	sh := map[string]interface{}{"flag": true}
	shb, _ := json.Marshal(sh)
	_ = s.UpdateTaskProgress(nt.ID, nt.CurrentNodeKey, "", string(shb), nt.StepCount)
	for i := 0; i < 50; i++ {
		_ = e.RunOnce(tid)
		nt2, _ := s.GetTask(tid)
		if nt2.Status == "completed" || nt2.CurrentNodeKey == "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("not completed")
}

func TestApprovalNode(t *testing.T) {
	s := openTestStore(t)
	srv := startWorker(t, s)
	defer srv.Close()
	fid, err := s.CreateFlow("f11")
	if err != nil {
		t.Fatalf("%v", err)
	}
	def := map[string]interface{}{
		"start": "ap",
		"nodes": map[string]interface{}{
			"ap":  map[string]interface{}{"kind": "approval", "params": map[string]interface{}{"approval_key": "$shared.approval"}, "post": map[string]interface{}{"action_key": "approval"}},
			"end": map[string]interface{}{"kind": "executor", "service": "transform", "prep": map[string]interface{}{"input_key": "$params.val"}, "params": map[string]interface{}{"mul": 1.0}, "post": map[string]interface{}{"action_static": ""}},
		},
		"edges": []map[string]interface{}{{"from": "ap", "action": "approved", "to": "end"}},
	}
	b, _ := json.Marshal(def)
	vid, err := s.CreateFlowVersion(fid, 1, string(b), "published")
	if err != nil {
		t.Fatalf("%v", err)
	}
	p := map[string]interface{}{"val": 2.0}
	pb, _ := json.Marshal(p)
	tid, err := s.CreateTask(vid, string(pb), "", "ap")
	if err != nil {
		t.Fatalf("%v", err)
	}
	e := New(s)
	for i := 0; i < 5; i++ {
		_ = e.RunOnce(tid)
		time.Sleep(10 * time.Millisecond)
	}
	nt, _ := s.GetTask(tid)
	sh := map[string]interface{}{"approval": "approved"}
	shb, _ := json.Marshal(sh)
	_ = s.UpdateTaskProgress(nt.ID, nt.CurrentNodeKey, "", string(shb), nt.StepCount)
	for i := 0; i < 50; i++ {
		_ = e.RunOnce(tid)
		nt2, _ := s.GetTask(tid)
		if nt2.Status == "completed" || nt2.CurrentNodeKey == "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("not completed")
}
