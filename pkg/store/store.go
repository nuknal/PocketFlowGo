package store

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

func GenID(prefix string) string { return fmt.Sprintf("%s-%s", prefix, uuid.New().String()) }

func NowUnix() int64 { return time.Now().Unix() }

// Store defines the interface for data persistence.
type Store interface {
	// Worker Registry
	RegisterWorker(w WorkerInfo) error
	HeartbeatWorker(id string, url string, load int) error
	RefreshWorkersStatus(ttl int64) error
	ListWorkers(service string, ttl int64) ([]WorkerInfo, error)

	// Flow Management
	CreateFlow(name string, description string) (string, error)
	CreateFlowVersion(flowID string, version int, definitionJSON string, status string) (string, error)
	ListFlows(limit, offset int) ([]Flow, int64, error)
	ListFlowVersions(flowID string) ([]FlowVersion, error)
	LatestPublishedVersion(flowID string) (FlowVersion, error)
	GetFlowVersionByFlowIDAndVersion(flowID string, version int) (FlowVersion, error)
	GetFlowVersionByID(id string) (FlowVersion, error)

	// Task Management
	CreateTask(flowVersionID string, paramsJSON string, requestID string, startNode string) (string, error)
	GetTask(id string) (Task, error)
	LeaseNextTask(owner string, ttlSec int64) (Task, error)
	ExtendLease(id string, owner string, ttlSec int64) error
	UpdateTaskStatus(id string, status string) error
	UpdateTaskStatusOwned(id string, owner string, status string) error
	UpdateTaskProgress(id string, currentNode string, lastAction string, sharedJSON string, stepCount int) error
	UpdateTaskProgressOwned(id string, owner string, currentNode string, lastAction string, sharedJSON string, stepCount int) error
	ListTasks(status string, flowVersionID string, limit, offset int) ([]Task, int64, error)

	// Node Execution History
	SaveNodeRun(nr map[string]interface{}) error
	CreateNodeRun(nr map[string]interface{}) error
	UpdateNodeRun(id string, updates map[string]interface{}) error
	ListNodeRuns(taskID string) ([]NodeRun, error)
	GetNodeRun(id string) (NodeRun, error)

	// Queue Operations
	EnqueueTask(taskID, nodeKey, service, inputJSON string) (string, error)
	PollQueue(workerID string, services []string, timeoutSec int64) (QueueTask, error)
	CompleteQueueTask(queueID string) (string, error)
	FailQueueTask(queueID string) error
}

// WorkerInfo represents a registered worker node.
type WorkerInfo struct {
	ID            string   `json:"id"`
	URL           string   `json:"url"`
	Services      []string `json:"services"`
	Load          int      `json:"load"`
	LastHeartbeat int64    `json:"last_heartbeat"`
	Status        string   `json:"status"`
	Type          string   `json:"type"` // http, async, local
}

type Flow struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   int64  `json:"created_at"`
}

type FlowVersion struct {
	ID             string `json:"id"`
	FlowID         string `json:"flow_id"`
	Version        int    `json:"version"`
	DefinitionJSON string `json:"definition_json"`
	Status         string `json:"status"`
}

type Task struct {
	ID             string `json:"id"`
	FlowVersionID  string `json:"flow_version_id"`
	FlowID         string `json:"flow_id,omitempty"`
	FlowName       string `json:"flow_name,omitempty"`
	FlowVersion    int    `json:"flow_version,omitempty"`
	Status         string `json:"status"`
	ParamsJSON     string `json:"params_json"`
	SharedJSON     string `json:"shared_json"`
	CurrentNodeKey string `json:"current_node_key"`
	LastAction     string `json:"last_action"`
	StepCount      int    `json:"step_count"`
	RetryStateJSON string `json:"retry_state_json"`
	LeaseOwner     string `json:"lease_owner"`
	LeaseExpiry    int64  `json:"lease_expiry"`
	RequestID      string `json:"request_id"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

type NodeRun struct {
	ID             string `json:"id"`
	TaskID         string `json:"task_id"`
	NodeKey        string `json:"node_key"`
	AttemptNo      int    `json:"attempt_no"`
	Status         string `json:"status"`
	SubStatus      string `json:"sub_status"`
	BranchID       string `json:"branch_id"`
	PrepJSON       string `json:"prep_json"`
	ExecInputJSON  string `json:"exec_input_json"`
	ExecOutputJSON string `json:"exec_output_json"`
	ErrorText      string `json:"error_text"`
	Action         string `json:"action"`
	StartedAt      int64  `json:"started_at"`
	FinishedAt     int64  `json:"finished_at"`
	WorkerID       string `json:"worker_id"`
	WorkerURL      string `json:"worker_url"`
	LogPath        string `json:"log_path"`
}

// QueueTask represents a task in the persistent queue
type QueueTask struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	NodeKey   string `json:"node_key"`
	Service   string `json:"service"`
	InputJSON string `json:"input_json"`
	Status    string `json:"status"`
	WorkerID  string `json:"worker_id"`
	CreatedAt int64  `json:"created_at"`
	StartedAt int64  `json:"started_at"`
	TimeoutAt int64  `json:"timeout_at"`
}
