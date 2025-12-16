package engine

import (
	"github.com/nuknal/PocketFlowGo/internal/store"
)

// ExecutorInput encapsulates input parameters for execution.
type ExecutorInput struct {
	Task    store.Task             `json:"task"`
	Node    DefNode                `json:"node"`
	NodeKey string                 `json:"node_key"`
	Input   interface{}            `json:"input"`
	Params  map[string]interface{} `json:"params"`
}

// ExecutorResult encapsulates the result of execution.
type ExecutorResult struct {
	Result     interface{}
	WorkerID   string
	WorkerURL  string
	LogPath    string
	Error      error
	SkipRecord bool
}

// NodeRunInput encapsulates input parameters for running a node.
type NodeRunInput struct {
	Task    store.Task             `json:"task"`
	FlowDef FlowDef                `json:"flow_def"`
	Node    DefNode                `json:"node"`
	NodeKey string                 `json:"node_key"`
	Shared  map[string]interface{} `json:"shared"`
	Params  map[string]interface{} `json:"params"`
	Input   interface{}            `json:"input"`
}

// DefNode represents a node in the flow definition.
// It contains configuration for various node types like executors, choices, parallels, etc.
type DefNode struct {
	Kind     string `json:"kind"`
	Service  string `json:"service"`
	ExecType string `json:"exec_type"`
	Func     string `json:"func"`
	Script   struct {
		Cmd           string            `json:"cmd"`
		Args          []string          `json:"args"`
		TimeoutMillis int               `json:"timeout_ms"`
		Env           map[string]string `json:"env"`
		WorkDir       string            `json:"work_dir"`
		StdinMode     string            `json:"stdin_mode"`
		OutputMode    string            `json:"output_mode"`
	} `json:"script"`
	Params map[string]interface{} `json:"params"`
	Prep   struct {
		InputKey string            `json:"input_key"`
		InputMap map[string]string `json:"input_map"`
	} `json:"prep"`
	Post struct {
		OutputKey    string            `json:"output_key"`
		OutputMap    map[string]string `json:"output_map"`
		ActionStatic string            `json:"action_static"`
		ActionKey    string            `json:"action_key"`
	} `json:"post"`
	MaxRetries         int           `json:"max_retries"`
	WaitMillis         int           `json:"wait_ms"`
	MaxAttempts        int           `json:"max_attempts"`
	AttemptDelayMillis int           `json:"attempt_delay_ms"`
	WeightedByLoad     bool          `json:"weighted_by_load"`
	ParallelServices   []string      `json:"parallel_services"`
	ParallelExecs      []ExecSpec    `json:"parallel_execs"`
	ForeachExecs       []ExecSpec    `json:"foreach_execs"`
	ChoiceKey          string        `json:"choice_key"`
	DefaultAction      string        `json:"default_action"`
	Subflow            *EmbeddedFlow `json:"subflow"`
	SubflowExecs       []ExecSpec    `json:"subflow_execs"`
	ChoiceCases        []ChoiceCase  `json:"choice_cases"`
	ParallelMode       string        `json:"parallel_mode"`
	MaxParallel        int           `json:"max_parallel"`
	FailureStrategy    string        `json:"failure_strategy"`
}

// DefEdge represents a transition between nodes.
type DefEdge struct {
	From   string `json:"from"`
	Action string `json:"action"`
	To     string `json:"to"`
}

// FlowDef represents the entire flow definition.
type FlowDef struct {
	Start string             `json:"start"`
	Nodes map[string]DefNode `json:"nodes"`
	Edges []DefEdge          `json:"edges"`
}

// EmbeddedFlow represents a sub-flow definition.
type EmbeddedFlow struct {
	Start string             `json:"start"`
	Nodes map[string]DefNode `json:"nodes"`
	Edges []DefEdge          `json:"edges"`
}

// ChoiceCase represents a single case in a choice node.
type ChoiceCase struct {
	Action string                 `json:"action"`
	Expr   map[string]interface{} `json:"expr"`
}

// ExecSpec represents a specification for execution.
type ExecSpec struct {
	Service  string                 `json:"service"`
	ExecType string                 `json:"exec_type"`
	Func     string                 `json:"func"`
	Params   map[string]interface{} `json:"params"`
	Node     string                 `json:"node"`
	Index    int                    `json:"index"`
	Script   struct {
		Cmd           string            `json:"cmd"`
		Args          []string          `json:"args"`
		TimeoutMillis int               `json:"timeout_ms"`
		Env           map[string]string `json:"env"`
		WorkDir       string            `json:"work_dir"`
		StdinMode     string            `json:"stdin_mode"`
		OutputMode    string            `json:"output_mode"`
	} `json:"script"`
}
