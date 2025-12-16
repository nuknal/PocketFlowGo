package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	f "github.com/nuknal/PocketFlowGo/pkg/flow"
)

// QueueTask represents the task received from the queue
type QueueTask struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	NodeKey   string `json:"node_key"`
	Service   string `json:"service"`
	InputJSON string `json:"input_json"`
}

// QueueResult represents the result to report back
type QueueResult struct {
	QueueID string      `json:"queue_id"`
	Result  interface{} `json:"result"`
	Error   string      `json:"error"`
	LogPath string      `json:"log_path"`
	RunID   string      `json:"run_id,omitempty"`
}

// QueueUpdate represents an update to the running task
type QueueUpdate struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
}

var (
	schedulerURL = flag.String("server", "http://localhost:8070", "Scheduler API URL")
	workerID     = flag.String("id", "", "Worker ID (default: hostname-pid)")
	services     = flag.String("services", "async-transform", "Comma-separated list of services")
	logDir       = flag.String("log_dir", "./logs", "Directory to store execution logs")
)

func main() {
	flag.Parse()

	if *workerID == "" {
		host, _ := os.Hostname()
		*workerID = fmt.Sprintf("async-%s-%d", host, os.Getpid())
	}

	svcList := strings.Split(*services, ",")
	log.Printf("Starting Async Worker %s polling %s for services: %v", *workerID, *schedulerURL, svcList)

	// Register and Heartbeat
	regClient := &f.RegistryClient{BaseURL: *schedulerURL + "/api"}
	go func() {
		// Registration loop
		for {
			err := regClient.Register(f.WorkerInfo{
				ID:       *workerID,
				URL:      "queue",
				Services: svcList,
				Type:     "async",
			})
			if err != nil {
				log.Printf("Registration failed: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			log.Printf("Registered as async worker")
			break
		}
		// Heartbeat loop
		for {
			time.Sleep(5 * time.Second)
			if err := regClient.Heartbeat(*workerID, "queue", 0); err != nil {
				log.Printf("Heartbeat error: %v", err)
			}
		}
	}()

	client := &http.Client{Timeout: 30 * time.Second}

	for {
		// 1. Poll
		task, err := poll(client, svcList)
		if err != nil {
			log.Printf("Poll error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		if task == nil {
			time.Sleep(1 * time.Second) // No tasks, wait
			continue
		}

		log.Printf("Received task %s for service %s", task.ID, task.Service)

		// 2. Execute with logging
		res, logPath, execErr := executeWithLog(task)
		errStr := ""
		if execErr != nil {
			errStr = execErr.Error()
			log.Printf("Task %s failed: %v", task.ID, execErr)
		} else {
			log.Printf("Task %s completed", task.ID)
		}

		// 3. Complete
		if err := completeTask(client, task, res, errStr, logPath); err != nil {
			log.Printf("Complete error: %v", err)
		}
	}
}

func executeWithLog(task *QueueTask) (interface{}, string, error) {
	// Parse payload to find run_id
	var payload struct {
		Input  interface{}            `json:"input"`
		Params map[string]interface{} `json:"params"`
		RunID  string                 `json:"run_id"`
	}
	_ = json.Unmarshal([]byte(task.InputJSON), &payload)

	// Create log dir if not exists
	if err := os.MkdirAll(*logDir, 0755); err != nil {
		return nil, "", fmt.Errorf("failed to create log dir: %v", err)
	}

	// Create log file
	absLogDir, _ := filepath.Abs(*logDir)
	logFileName := fmt.Sprintf("%s_%d.log", task.ID, time.Now().UnixNano())
	logPath := filepath.Join(absLogDir, logFileName)

	f, err := os.Create(logPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create log file: %v", err)
	}
	defer f.Close()

	// Create a multi-writer logger
	mw := io.MultiWriter(os.Stdout, f)
	logger := log.New(mw, fmt.Sprintf("[%s] ", task.ID), log.LstdFlags)

	logger.Printf("Starting execution for task %s (Service: %s)", task.ID, task.Service)

	// Report "running" status if run_id exists
	if payload.RunID != "" {
		updateStatus(payload.RunID, "running")
	}

	res, err := execute(task, logger)
	logger.Printf("Execution finished. Error: %v", err)

	return res, logPath, err
}

func updateStatus(runID string, status string) {
	client := &http.Client{Timeout: 5 * time.Second}
	reqBody := QueueUpdate{
		RunID:  runID,
		Status: status,
	}
	b, _ := json.Marshal(reqBody)
	resp, err := client.Post(*schedulerURL+"/api/queue/update_run", "application/json", bytes.NewReader(b))
	if err != nil {
		log.Printf("Failed to update status: %v", err)
		return
	}
	defer resp.Body.Close()
}

func poll(client *http.Client, services []string) (*QueueTask, error) {
	reqBody := map[string]interface{}{
		"worker_id": *workerID,
		"services":  services,
	}
	b, _ := json.Marshal(reqBody)
	resp, err := client.Post(*schedulerURL+"/api/queue/poll", "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("poll status %d", resp.StatusCode)
	}

	var task QueueTask
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, err
	}
	return &task, nil
}

func complete(client *http.Client, queueID string, result interface{}, errStr string, logPath string) error {
	// Need to extract RunID again or pass it down.
	// Since we are not passing task object here, let's keep it simple.
	// We can update QueueResult struct in main.go but we also need to pass RunID to complete.
	// Let's assume complete just calls the endpoint and the endpoint handles the rest.
	// But wait, the endpoint uses RunID if present to update the run.

	// Actually, we should extract RunID in main loop and pass it.
	return nil
}

// Rewriting complete function to be cleaner and use RunID
func completeTask(client *http.Client, task *QueueTask, result interface{}, errStr string, logPath string) error {
	var payload struct {
		RunID string `json:"run_id"`
	}
	_ = json.Unmarshal([]byte(task.InputJSON), &payload)

	reqBody := QueueResult{
		QueueID: task.ID,
		Result:  result,
		Error:   errStr,
		LogPath: logPath,
		RunID:   payload.RunID,
	}
	b, _ := json.Marshal(reqBody)
	resp, err := client.Post(*schedulerURL+"/api/queue/complete", "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("complete status %d", resp.StatusCode)
	}
	return nil
}

func execute(task *QueueTask, logger *log.Logger) (interface{}, error) {
	// Parse input
	var payload struct {
		Input  interface{}            `json:"input"`
		Params map[string]interface{} `json:"params"`
	}
	if err := json.Unmarshal([]byte(task.InputJSON), &payload); err != nil {
		return nil, fmt.Errorf("invalid input json: %v", err)
	}

	// Simulate processing time
	logger.Printf("Processing %s...", task.Service)
	time.Sleep(2 * time.Second)

	// Basic logic
	switch task.Service {
	case "async-transform":
		logger.Printf("payload: %v", payload)
		if inputMap, ok := payload.Input.(map[string]interface{}); ok {
			if text, ok := inputMap["text"].(string); ok {
				title := inputMap["title"]
				return fmt.Sprintf("[%v] %s", title, strings.ToUpper(text)), nil
			}
		}

		if s, ok := payload.Input.(string); ok {
			return strings.ToUpper(s) + " (ASYNC PROCESSED)", nil
		}
		if s, ok := payload.Params["text"].(string); ok {
			return strings.ToUpper(s) + " (ASYNC PROCESSED)", nil
		}
		return nil, fmt.Errorf("expected string input or params['text']")
	case "email-service":
		to, _ := payload.Params["to"].(string)
		return map[string]string{"status": "sent", "to": to}, nil
	default:
		return nil, fmt.Errorf("unknown service: %s", task.Service)
	}
}
