package engine

import (
	"testing"

	"github.com/nuknal/PocketFlowGo/internal/store"
)

func TestExecLocalScript_Code(t *testing.T) {
	// Mock engine
	e := &Engine{}

	// Case 1: Python script
	node := DefNode{
		Kind: "executor",
		Func: "local_script",
	}
	node.Script.Language = "python"
	node.Script.Code = `
import sys
import json

print(json.dumps({"msg": "hello from python"}))
`
	node.Script.OutputMode = "json"

	input := ExecutorInput{
		Task:    store.Task{ID: "test-task"},
		Node:    node,
		NodeKey: "test-node",
	}

	res := e.execLocalScript(input)
	if res.Error != nil {
		t.Fatalf("Python execution failed: %v", res.Error)
	}

	resMap, ok := res.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T: %v", res.Result, res.Result)
	}
	if resMap["msg"] != "hello from python" {
		t.Errorf("Unexpected result: %v", resMap)
	}

	// Case 2: Shell script
	nodeSh := DefNode{
		Kind: "executor",
		Func: "local_script",
	}
	nodeSh.Script.Language = "bash"
	nodeSh.Script.Code = `
echo "hello from bash"
`
	inputSh := ExecutorInput{
		Task:    store.Task{ID: "test-task-sh"},
		Node:    nodeSh,
		NodeKey: "test-node-sh",
	}

	resSh := e.execLocalScript(inputSh)
	if resSh.Error != nil {
		t.Fatalf("Bash execution failed: %v", resSh.Error)
	}

	resStr, ok := resSh.Result.(string)
	if !ok {
		t.Fatalf("Expected string result, got %T: %v", resSh.Result, resSh.Result)
	}
	if resStr != "hello from bash\n" {
		t.Errorf("Unexpected result: %q", resStr)
	}
}
