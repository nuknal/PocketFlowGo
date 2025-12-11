package engine

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

type DefEdge struct {
	From   string `json:"from"`
	Action string `json:"action"`
	To     string `json:"to"`
}

type FlowDef struct {
	Start string             `json:"start"`
	Nodes map[string]DefNode `json:"nodes"`
	Edges []DefEdge          `json:"edges"`
}

type EmbeddedFlow struct {
	Start string             `json:"start"`
	Nodes map[string]DefNode `json:"nodes"`
	Edges []DefEdge          `json:"edges"`
}

type ChoiceCase struct {
	Action string                 `json:"action"`
	Expr   map[string]interface{} `json:"expr"`
}

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
