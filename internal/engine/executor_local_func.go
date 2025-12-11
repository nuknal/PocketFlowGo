package engine

import (
    "context"
    "time"
)

func (e *Engine) execLocalFunc(node DefNode, input interface{}, params map[string]interface{}) (interface{}, string, string, error) {
    attempts := 0
    fn := e.LocalFuncs[node.Func]
    if fn == nil {
        return nil, "", "", errorString("no func")
    }
    for {
        attempts++
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        res, err := fn(ctx, input, params)
        cancel()
        if err != nil {
            if node.AttemptDelayMillis > 0 {
                time.Sleep(time.Duration(node.AttemptDelayMillis) * time.Millisecond)
            }
            if node.MaxAttempts > 0 && attempts >= node.MaxAttempts {
                break
            }
            continue
        }
        return res, "local-func:" + node.Func, "local", nil
    }
    return nil, "", "", errorString("failed")
}
