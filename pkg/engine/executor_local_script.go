package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// execLocalScript executes a local shell command or script.
// It supports setting working directory, environment variables, timeout, and stdin/stdout formats.
func (e *Engine) execLocalScript(in ExecutorInput) ExecutorResult {
	attempts := 0
	for {
		attempts++
		to := 10 * time.Second
		if in.Node.Script.TimeoutMillis > 0 {
			to = time.Duration(in.Node.Script.TimeoutMillis) * time.Millisecond
		}
		ctx, cancel := context.WithTimeout(context.Background(), to)

		var cmd *exec.Cmd
		var tempFile string

		// If code is provided, write it to a temp file
		if in.Node.Script.Code != "" {
			ext := ".sh"
			shell := "bash"

			// Detect language/interpreter
			if in.Node.Script.Language != "" {
				switch in.Node.Script.Language {
				case "python":
					ext = ".py"
					shell = "python3"
				case "javascript", "node":
					ext = ".js"
					shell = "node"
				case "go":
					ext = ".go"
					shell = "go run"
				default:
					shell = in.Node.Script.Language
				}
			} else {
				// Infer from cmd if present
				if in.Node.Script.Cmd != "" {
					shell = in.Node.Script.Cmd
				}
			}

			// Create temp file
			f, err := os.CreateTemp("", "pf-script-*"+ext)
			if err != nil {
				cancel()
				return ExecutorResult{Error: err}
			}
			f.WriteString(in.Node.Script.Code)
			f.Close()
			tempFile = f.Name()

			// Build command args
			// If shell has spaces (e.g. "go run"), split it
			// This is a naive split, but sufficient for standard interpreters
			// For complex cases, users should use explicit Cmd + Code

			// Construct command
			// e.g. bash /tmp/file
			// e.g. python3 /tmp/file

			// We need to handle arguments passed to the script as well
			// in.Node.Script.Args are arguments TO the script

			// Command structure: [interpreter] [interpreter_flags] [script_path] [script_args]

			runCmd := shell
			runArgs := []string{}

			if in.Node.Script.Cmd != "" {
				runCmd = in.Node.Script.Cmd
				// If cmd is set, we assume it's the interpreter
			}

			runArgs = append(runArgs, tempFile)
			runArgs = append(runArgs, in.Node.Script.Args...)

			cmd = exec.CommandContext(ctx, runCmd, runArgs...)
		} else {
			cmd = exec.CommandContext(ctx, in.Node.Script.Cmd, in.Node.Script.Args...)
		}

		// Configure execution environment
		if in.Node.Script.WorkDir != "" {
			cmd.Dir = in.Node.Script.WorkDir
		}
		if in.Node.Script.Env != nil {
			env := os.Environ()
			for k, v := range in.Node.Script.Env {
				env = append(env, k+"="+v)
			}
			cmd.Env = env
		}

		// Prepare input
		payload := map[string]interface{}{"input": in.Input, "params": in.Params}
		if in.Node.Script.StdinMode == "json" {
			b, _ := json.Marshal(payload)
			cmd.Stdin = bytes.NewReader(b)
		}

		outb, err := cmd.CombinedOutput()
		cancel()

		// Cleanup temp file
		if tempFile != "" {
			_ = os.Remove(tempFile)
		}

		// Save logs
		logDir := filepath.Join("logs", "tasks", in.Task.ID)
		_ = os.MkdirAll(logDir, 0755)
		logPath := filepath.Join(logDir, fmt.Sprintf("%s_%d.log", in.NodeKey, attempts))
		_ = os.WriteFile(logPath, outb, 0644)

		// Handle error and retries
		if err != nil {
			if in.Node.AttemptDelayMillis > 0 {
				time.Sleep(time.Duration(in.Node.AttemptDelayMillis) * time.Millisecond)
			}
			if in.Node.MaxAttempts == 0 || (in.Node.MaxAttempts > 0 && attempts >= in.Node.MaxAttempts) {
				break
			}
			continue
		}

		// Parse output
		var res interface{}
		if in.Node.Script.OutputMode == "json" {
			var v interface{}
			if json.Unmarshal(outb, &v) == nil {
				res = v
			} else {
				res = string(outb)
			}
		} else {
			res = string(outb)
		}
		return ExecutorResult{Result: res, WorkerID: "local-script:" + in.Node.Script.Cmd, WorkerURL: "local", LogPath: logPath}
	}
	return ExecutorResult{Error: errorString("failed")}
}
