package engine

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "sort"
    "time"
)

// execHTTP executes an HTTP request to a worker service.
// It performs service discovery, load balancing, and retries across available workers.
func (e *Engine) execHTTP(node DefNode, input interface{}, params map[string]interface{}) (interface{}, string, string, error) {
    // 1. Discover available workers
    lst, _ := e.Store.ListWorkers(node.Service, 15)
    if len(lst) == 0 {
        return nil, "", "", errorString("no worker")
    }

    // 2. Load balance (optionally weighted by load)
    if node.WeightedByLoad {
        sort.SliceStable(lst, func(i, j int) bool { return lst[i].Load < lst[j].Load })
    }

    payload := map[string]interface{}{"input": input, "params": params}
    b, _ := json.Marshal(payload)
    attempts := 0

    // 3. Try execution on workers
    for _, w := range lst {
        attempts++
        endpoint := w.URL + "/exec/" + node.Service
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(b))
        if err != nil {
            cancel()
            continue
        }
        req.Header.Set("Content-Type", "application/json")
        resp, err := e.HTTP.Do(req)
        if err != nil {
            cancel()
            continue
        }
        var out struct {
            Result interface{} `json:"result"`
            Error  string      `json:"error"`
        }
        dec := json.NewDecoder(resp.Body)
        if err := dec.Decode(&out); err != nil {
            resp.Body.Close()
            cancel()
            continue
        }
        resp.Body.Close()
        cancel()
        
        // Handle worker error
        if out.Error != "" {
            if node.AttemptDelayMillis > 0 {
                time.Sleep(time.Duration(node.AttemptDelayMillis) * time.Millisecond)
            }
            if node.MaxAttempts > 0 && attempts >= node.MaxAttempts {
                break
            }
            continue
        }
        return out.Result, w.ID, w.URL, nil
    }
    return nil, "", "", errorString("all workers failed")
}
