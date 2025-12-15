package engine

import (
	"context"
	"time"
)

// execLocalFunc executes a registered local Go function.
// It is useful for lightweight tasks that don't require a separate worker service.
func (e *Engine) execLocalFunc(in ExecutorInput) ExecutorResult {
	attempts := 0
	fn := e.LocalFuncs[in.Node.Func]
	if fn == nil {
		return ExecutorResult{Error: ErrFatal}
	}

	// Retry loop
	for {
		attempts++
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		res, err := fn(ctx, in.Input, in.Params)
		cancel()

		if err != nil {
			if in.Node.AttemptDelayMillis > 0 {
				time.Sleep(time.Duration(in.Node.AttemptDelayMillis) * time.Millisecond)
			}
			if in.Node.MaxAttempts == 0 || (in.Node.MaxAttempts > 0 && attempts >= in.Node.MaxAttempts) {
				break
			}

			continue
		}
		return ExecutorResult{Result: res, WorkerID: "local-func:" + in.Node.Func, WorkerURL: "local"}
	}
	return ExecutorResult{WorkerID: "local-func:" + in.Node.Func, WorkerURL: "local", Error: errorString("failed")}
}
