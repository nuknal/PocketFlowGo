package engine

import (
	"context"
	"time"
)

// execLocalFunc executes a registered local Go function.
// It is useful for lightweight tasks that don't require a separate worker service.
func (e *Engine) execLocalFunc(node DefNode, input interface{}, params map[string]interface{}) (interface{}, string, string, error) {
	attempts := 0
	fn := e.LocalFuncs[node.Func]
	if fn == nil {
		return nil, "", "", ErrFatal
	}

	// Retry loop
	for {
		attempts++
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		res, err := fn(ctx, input, params)
		cancel()

		if err != nil {
			if node.AttemptDelayMillis > 0 {
				time.Sleep(time.Duration(node.AttemptDelayMillis) * time.Millisecond)
			}
			if node.MaxAttempts == 0 || (node.MaxAttempts > 0 && attempts >= node.MaxAttempts) {
				break
			}

			continue
		}
		return res, "local-func:" + node.Func, "local", nil
	}
	return nil, "local-func:" + node.Func, "local", errorString("failed")
}
