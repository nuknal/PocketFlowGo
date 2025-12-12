package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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

	if len(os.Args) < 2 {
		fmt.Println("Usage: cli <command> [args]")
		fmt.Println("Commands: create")
		return
	}

	cmd := os.Args[1]
	switch cmd {
	case "create":
		handleCreate(base)
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
	}
}

func handleCreate(base string) {
	var flowFile string
	var paramsJSON string

	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		if args[i] == "-f" && i+1 < len(args) {
			flowFile = args[i+1]
			i++
		} else if args[i] == "-p" && i+1 < len(args) {
			paramsJSON = args[i+1]
			i++
		}
	}

	if flowFile == "" {
		fmt.Println("Usage: cli create -f <flow.json> [-p <params_json>]")
		return
	}

	// Read flow definition
	defBytes, err := os.ReadFile(flowFile)
	if err != nil {
		fmt.Printf("Failed to read flow file: %v\n", err)
		return
	}

	// Validate JSON
	var defMap map[string]interface{}
	if err := json.Unmarshal(defBytes, &defMap); err != nil {
		fmt.Printf("Invalid JSON in flow file: %v\n", err)
		return
	}

	// Create Flow
	name := filepath.Base(flowFile)
	var flowResp map[string]string
	if err := postJSON(base, "/flows", map[string]string{"Name": name}, &flowResp); err != nil {
		fmt.Printf("Create Flow failed: %v\n", err)
		return
	}
	flowID := flowResp["id"]
	fmt.Printf("Created Flow: %s (ID: %s)\n", name, flowID)

	// Create Version
	var verResp map[string]string
	if err := postJSON(base, "/flows/version", map[string]interface{}{
		"FlowID":         flowID,
		"Version":        1,
		"DefinitionJSON": string(defBytes),
		"Status":         "published",
	}, &verResp); err != nil {
		fmt.Printf("Create Version failed: %v\n", err)
		return
	}
	verID := verResp["id"]
	fmt.Printf("Created Version: 1 (ID: %s)\n", verID)

	// Create Task
	if paramsJSON == "" {
		paramsJSON = "{}"
	}
	var tResp map[string]string
	if err := postJSON(base, "/tasks", map[string]interface{}{
		"FlowVersionID": verID,  // Use FlowVersionID explicitly
		"FlowID":        flowID, // Fallback
		"ParamsJSON":    paramsJSON,
	}, &tResp); err != nil {
		fmt.Printf("Create Task failed: %v\n", err)
		return
	}
	taskID := tResp["id"]
	fmt.Printf("Created Task: %s\n", taskID)

	// Poll for status
	monitorTask(base, taskID)
}

func monitorTask(base, taskID string) {
	fmt.Println("Monitoring task...")
	for i := 0; i < 60; i++ { // Poll for 60 seconds
		var gt struct {
			ID             string
			Status         string
			CurrentNodeKey string
			SharedJSON     string
		}
		if err := getJSON(base, "/tasks/get?id="+taskID, &gt); err != nil {
			fmt.Printf("Get Task failed: %v\n", err)
			time.Sleep(1 * time.Second)
			continue
		}
		fmt.Printf("[%s] Status: %s, Node: %s\n", time.Now().Format("15:04:05"), gt.Status, gt.CurrentNodeKey)

		if gt.Status == "completed" || gt.Status == "failed" {
			fmt.Println("Final Shared State:", gt.SharedJSON)
			break
		}
		time.Sleep(1 * time.Second)
	}
}
