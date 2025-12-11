package engine

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "sort"
    "time"
)

func (e *Engine) execHTTP(node DefNode, input interface{}, params map[string]interface{}) (interface{}, string, string, error) {
    lst, _ := e.Store.ListWorkers(node.Service, 15)
    if len(lst) == 0 {
        return nil, "", "", errorString("no worker")
    }
    if node.WeightedByLoad {
        sort.SliceStable(lst, func(i, j int) bool { return lst[i].Load < lst[j].Load })
    }
    payload := map[string]interface{}{"input": input, "params": params}
    b, _ := json.Marshal(payload)
    attempts := 0
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
