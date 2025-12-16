package engine

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "sort"
    "time"

    "github.com/nuknal/PocketFlowGo/pkg/store"
)

// execHTTP executes an HTTP request to a worker service.
// It performs service discovery, load balancing, and retries across available workers.
func (e *Engine) execHTTP(in ExecutorInput) ExecutorResult {
	// 1. Discover available workers
	lst, _ := e.Store.ListWorkers(in.Node.Service, 15)

	// Filter for HTTP workers only
	var httpWorkers []store.WorkerInfo
	for _, w := range lst {
		if w.Type == "http" || w.Type == "" {
			httpWorkers = append(httpWorkers, w)
		}
	}
	lst = httpWorkers

	if len(lst) == 0 {
		return ExecutorResult{Error: errorString("no worker")}
	}

	// 2. Load balance (optionally weighted by load)
	if in.Node.WeightedByLoad {
		sort.SliceStable(lst, func(i, j int) bool { return lst[i].Load < lst[j].Load })
	}

	payload := map[string]interface{}{"input": in.Input, "params": in.Params}
	b, _ := json.Marshal(payload)
	attempts := 0

	// 3. Try execution on workers
	for _, w := range lst {
		attempts++
		endpoint := w.URL + "/exec/" + in.Node.Service
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
		if json.NewDecoder(resp.Body).Decode(&out) == nil {
			resp.Body.Close()
			if out.Error != "" {
				return ExecutorResult{WorkerID: w.ID, WorkerURL: w.URL, Error: errorString(out.Error)}
			}
			return ExecutorResult{Result: out.Result, WorkerID: w.ID, WorkerURL: w.URL}
		}
		resp.Body.Close()
	}
	return ExecutorResult{Error: errorString("all workers failed")}
}
