package engine

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/nuknal/PocketFlowGo/internal/store"
)

func TestExecutorQueue_Basic(t *testing.T) {
	// Setup DB
	dbPath := "test_queue_basic.db"
	os.Remove(dbPath)
	defer os.Remove(dbPath)
	s, err := store.OpenSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	// Setup Engine
	eng := New(s)
	eng.Owner = "tester"

	// Define Flow with one Queue Node
	flowDef := `{
		"start": "node1",
		"nodes": {
			"node1": {
				"kind": "executor",
				"exec_type": "queue",
				"service": "async-worker",
				"params": {"p1": "v1"},
				"post": {"action_static": "next"}
			}
		},
		"edges": [
			{"from": "node1", "action": "next", "to": "node2"},
			{"from": "node2", "action": "default", "to": ""}
		]
	}`

	// Create Flow & Version
	fid, _ := s.CreateFlow("queue-flow", "desc")
	vid, _ := s.CreateFlowVersion(fid, 1, flowDef, "published")

	// Create Task
	tid, err := s.CreateTask(vid, "{}", "req-1", "node1")
	if err != nil {
		t.Fatal(err)
	}

	// Lease the task (RunOnce requires a valid lease)
	_, err = s.LeaseNextTask(eng.Owner, 10)
	if err != nil {
		t.Fatalf("LeaseNextTask failed: %v", err)
	}

	// 1. First Run: Should suspend
	err = eng.RunOnce(tid)
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	task, _ := s.GetTask(tid)
	if task.Status != "waiting_queue" {
		t.Fatalf("Expected status waiting_queue, got %s", task.Status)
	}

	// Verify Task Queue entry
	qTask, err := s.PollQueue("w1", []string{"async-worker"}, 60)
	if err != nil {
		t.Fatal(err)
	}
	if qTask.ID == "" {
		t.Fatal("Expected queue task, got none")
	}
	if qTask.TaskID != tid {
		t.Errorf("Expected queue task for task %s, got %s", tid, qTask.TaskID)
	}

	// 2. Worker Completes Task
	// Simulate what API does: Save Node Run -> Update Task Status to Pending
	runResult := map[string]interface{}{"output": "done"}
	outBytes, _ := json.Marshal(runResult)

	run := map[string]interface{}{
		"task_id":          tid,
		"node_key":         "node1",
		"attempt_no":       1,
		"status":           "ok",
		"prep_json":        "{}",
		"exec_input_json":  qTask.InputJSON,
		"exec_output_json": string(outBytes),
		"error_text":       "",
		"action":           "",
		"started_at":       time.Now().Unix(),
		"finished_at":      time.Now().Unix(),
		"worker_id":        "w1",
		"worker_url":       "queue",
	}
	if err := s.SaveNodeRun(run); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompleteQueueTask(qTask.ID); err != nil { // Mark queue item completed
		t.Fatal(err)
	}
	if err := s.UpdateTaskStatus(tid, "pending"); err != nil { // Resume flow task
		t.Fatal(err)
	}
	// We need to re-lease the task because status change might have cleared or invalidated lease logic
	// (Actually UpdateTaskStatus clears lease owner? No, UpdateTaskStatus doesn't check lease, but resets status)
	// However, LeaseNextTask query checks for (lease_expiry=0 OR lease_expiry<now).
	// If the previous lease is still valid, LeaseNextTask won't pick it up unless we force expire it or use ExtendLease.
	// But since we are testing RunOnce directly, we just need to ensure RunOnce passes lease check.
	// The previous lease is valid for 10s.
	// But wait, UpdateTaskStatus updates updated_at.
	// RunOnce checks: if e.Owner != "" { if t.LeaseOwner != e.Owner ... }
	// The lease owner is still "tester".

	// 3. Second Run: Should Resume and Finish
	err = eng.RunOnce(tid)
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	task, _ = s.GetTask(tid)
	// It should have moved past node1.
	// In our flow, node1 -> node2. node2 is not defined in nodes map?
	// Wait, if node2 is missing in map but present in edge to, finishNode might set it as current?
	// Or it might fail if node2 definition is missing.
	// Let's add node2 to be safe, or check if it completed if node2 is end.
	// Actually, if node2 is not in nodes map, Engine usually stops or errors?
	// Let's check logic: Engine reads node definition. If missing, it might error.
	// Let's update flowDef to include node2 as a simple end node (or just check if it moved to node2)

	if task.CurrentNodeKey != "node2" {
		t.Errorf("Expected current node node2, got %s", task.CurrentNodeKey)
	}

	// Since node2 is not defined, next run would fail or stop. But we just wanted to verify node1 completion.
	// Let's verify shared state has output if we mapped it (we didn't in this test).
}
