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
func (e *Engine) execLocalScript(taskID, nodeKey string, node DefNode, input interface{}, params map[string]interface{}) (interface{}, string, string, string, error) {
	attempts := 0
	for {
		attempts++
		to := 10 * time.Second
		if node.Script.TimeoutMillis > 0 {
			to = time.Duration(node.Script.TimeoutMillis) * time.Millisecond
		}
		ctx, cancel := context.WithTimeout(context.Background(), to)
		cmd := exec.CommandContext(ctx, node.Script.Cmd, node.Script.Args...)

		// Configure execution environment
		if node.Script.WorkDir != "" {
			cmd.Dir = node.Script.WorkDir
		}
		if node.Script.Env != nil {
			env := os.Environ()
			for k, v := range node.Script.Env {
				env = append(env, k+"="+v)
			}
			cmd.Env = env
		}

		// Prepare input
		payload := map[string]interface{}{"input": input, "params": params}
		if node.Script.StdinMode == "json" {
			b, _ := json.Marshal(payload)
			cmd.Stdin = bytes.NewReader(b)
		}

		outb, err := cmd.CombinedOutput()
		cancel()

		// Save logs
		logDir := filepath.Join("logs", "tasks", taskID)
		_ = os.MkdirAll(logDir, 0755)
		logPath := filepath.Join(logDir, fmt.Sprintf("%s_%d.log", nodeKey, attempts))
		_ = os.WriteFile(logPath, outb, 0644)

		// Handle error and retries
		if err != nil {
			if node.AttemptDelayMillis > 0 {
				time.Sleep(time.Duration(node.AttemptDelayMillis) * time.Millisecond)
			}
			if node.MaxAttempts == 0 || (node.MaxAttempts > 0 && attempts >= node.MaxAttempts) {
				break
			}
			continue
		}

		// Parse output
		var res interface{}
		if node.Script.OutputMode == "json" {
			var v interface{}
			if json.Unmarshal(outb, &v) == nil {
				res = v
			} else {
				res = string(outb)
			}
		} else {
			res = string(outb)
		}
		return res, "local-script:" + node.Script.Cmd, "local", logPath, nil
	}
	return nil, "", "", "", errorString("failed")
}
