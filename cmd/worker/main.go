package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	f "github.com/nuknal/PocketFlowGo/pkg/flow"
)

type execRequest struct {
	Input  interface{}            `json:"input"`
	Params map[string]interface{} `json:"params"`
}

type execResponse struct {
	Result interface{} `json:"result"`
	Error  string      `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, resp execResponse) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

var currentLoad int64

func decodeReq(r *http.Request) (execRequest, error) {
	var req execRequest
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&req)
	return req, err
}

func transformHandler(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&currentLoad, 1)
	defer atomic.AddInt64(&currentLoad, -1)
	req, err := decodeReq(r)
	if err != nil {
		writeJSON(w, execResponse{Error: "bad request"})
		return
	}
	op, _ := req.Params["op"].(string)
	switch v := req.Input.(type) {
	case string:
		if op == "upper" {
			writeJSON(w, execResponse{Result: strings.ToUpper(v)})
			return
		}
		if op == "lower" {
			writeJSON(w, execResponse{Result: strings.ToLower(v)})
			return
		}
		writeJSON(w, execResponse{Error: "unsupported op"})
		return
	case float64:
		mul := 1.0
		if m, ok := req.Params["mul"].(float64); ok {
			mul = m
		}
		log.Printf("transform op=%s input=%f mul=%f", op, v, mul)
		writeJSON(w, execResponse{Result: v * mul})
		return
	default:
		writeJSON(w, execResponse{Error: "bad input"})
		return
	}
}

func sumHandler(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&currentLoad, 1)
	defer atomic.AddInt64(&currentLoad, -1)
	req, err := decodeReq(r)
	if err != nil {
		writeJSON(w, execResponse{Error: "bad request"})
		return
	}
	switch arr := req.Input.(type) {
	case []interface{}:
		s := 0.0
		for _, x := range arr {
			if f, ok := x.(float64); ok {
				s += f
				continue
			}
			if sstr, ok := x.(string); ok {
				if parsed, perr := strconv.ParseFloat(sstr, 64); perr == nil {
					s += parsed
				}
			}
		}
		writeJSON(w, execResponse{Result: s})
		return
	default:
		writeJSON(w, execResponse{Error: "bad input"})
		return
	}
}

func routeHandler(w http.ResponseWriter, r *http.Request) {
	req, err := decodeReq(r)
	if err != nil {
		writeJSON(w, execResponse{Error: "bad request"})
		return
	}
	act := "goB"
	if v, ok := req.Params["action"].(string); ok && v != "" {
		act = v
	}
	writeJSON(w, execResponse{Result: map[string]interface{}{"action": act}})
}

func main() {
	regURL := os.Getenv("REGISTRY_URL")
	if regURL == "" {
		regURL = "http://localhost:8070/api"
	}
	selfURL := os.Getenv("WORKER_URL")
	if selfURL == "" {
		selfURL = "http://localhost:8080"
	}
	client := &f.RegistryClient{BaseURL: regURL}
	id := fmt.Sprintf("worker-%d", time.Now().UnixNano())
	mux := http.NewServeMux()
	services := map[string]http.HandlerFunc{
		"transform": transformHandler,
		"sum":       sumHandler,
		"route":     routeHandler,
	}
	for name, h := range services {
		mux.HandleFunc("/exec/"+name, h)
	}
	bind := ":8080"
	var u *url.URL
	if uu, err := url.Parse(selfURL); err == nil {
		u = uu
		if p := uu.Port(); p != "" {
			bind = ":" + p
		}
	}
	ln, err := net.Listen("tcp", bind)
	if err != nil {
		ln, _ = net.Listen("tcp", ":0")
	}
	if u == nil {
		u, _ = url.Parse("http://localhost")
	}
	host := u.Hostname()
	if host == "" {
		host = "localhost"
	}
	port := ln.Addr().(*net.TCPAddr).Port
	u.Host = host + ":" + strconv.Itoa(port)
	selfURL = u.String()
	for {
		if err := client.Register(f.WorkerInfo{ID: id, URL: selfURL, Services: keys(services), Type: "http"}); err != nil {
			log.Printf("register error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
	go func() {
		for {
			time.Sleep(5 * time.Second)
			if err := client.Heartbeat(id, selfURL, int(atomic.LoadInt64(&currentLoad))); err != nil {
				log.Printf("heartbeat error: %v", err)
			}
		}
	}()
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	_ = srv.Serve(ln)
}

func keys(m map[string]http.HandlerFunc) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
