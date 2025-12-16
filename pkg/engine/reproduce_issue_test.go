package engine

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParallelNode_UpperFunc_MissingInput(t *testing.T) {
	s := openTestStore(t)
	// We don't need external workers for this test as we will register local func

	fid, err := s.CreateFlow("f_upper_fail", "")
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Define flow with parallel node that calls 'upper'
	def := map[string]interface{}{
		"start": "para",
		"nodes": map[string]interface{}{
			"para": map[string]interface{}{
				"kind":              "parallel",
				"parallel_mode":     "concurrent",
				"parallel_services": []string{"upper_svc"},
				"parallel_execs": []map[string]interface{}{
					{
						"service":   "upper_svc",
						"exec_type": "local_func",
						"func":      "upper",
						// We deliberately OMIT "prep" or pass empty params to trigger missing input error
						// But wait, "prep" defaults to empty.
						// The issue description says: "without input { "text": "..." }"
						// If we don't provide input, `upper` receives nil or empty map?
						// Let's see how `upper` is implemented in scheduler/main.go
					},
				},
				"post": map[string]interface{}{"action_static": "next"},
			},
			"end": map[string]interface{}{
				"kind":      "executor",
				"exec_type": "local_func",
				"func":      "log_result",
				"post":      map[string]interface{}{"action_static": ""},
			},
		},
		"edges": []map[string]interface{}{
			{"from": "para", "action": "next", "to": "end"},
		},
	}
	b, _ := json.Marshal(def)
	vid, err := s.CreateFlowVersion(fid, 1, string(b), "published")
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Create task with NO params that `upper` expects
	tid, err := s.CreateTask(vid, "{}", "req-fail", "para")
	if err != nil {
		t.Fatalf("%v", err)
	}

	e := New(s)
	// Register 'upper' func similar to scheduler/main.go
	e.RegisterFunc("upper", UpperFunc)

	// Run
	// We expect it to FAIL because `upper` returns error "expected string input"
	// And parallel node should capture that error.
	// Since default failure_strategy is NOT "continue" (it defaults to empty -> implies fail_fast?),
	// actually parallel.go:54 defaults strategy to node.FailureStrategy.
	// If node.FailureStrategy is empty, it's treated as fail?
	// In parallel.go:
	// "strat := node.FailureStrategy"
	// "if strat == "fail_fast" && hadErr { ... }"
	// Wait, if strat is empty, it falls through?
	// And then it loops again? Or returns nil?
	// If it returns nil without updating status to failed/completed, it might get stuck in "running" loop?

	// Let's see what happens.
	for i := 0; i < 20; i++ {
		err := e.RunOnce(tid)
		if err != nil {
			// RunOnce might return error if DB fails, but usually it swallows execution errors and updates Task status.
			t.Logf("RunOnce returned error: %v", err)
		}

		nt, _ := s.GetTask(tid)
		if nt.Status == "failed" {
			t.Log("Task successfully failed as expected")
			return
		}
		if nt.Status == "completed" {
			t.Fatalf("Task completed unexpectedly")
		}
		time.Sleep(10 * time.Millisecond)
	}

	nt, _ := s.GetTask(tid)
	t.Fatalf("Task did not fail. Status: %s, CurrentNode: %s", nt.Status, nt.CurrentNodeKey)
}
