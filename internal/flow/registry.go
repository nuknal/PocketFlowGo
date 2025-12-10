package flow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type WorkerInfo struct {
	ID            string   `json:"id"`
	URL           string   `json:"url"`
	Services      []string `json:"services"`
	Load          int      `json:"load"`
	LastHeartbeat int64    `json:"lastHeartbeat"`
}

type RegistryClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func (rc *RegistryClient) Register(w WorkerInfo) error {
	c := rc.HTTPClient
	if c == nil {
		c = &http.Client{}
	}
	b, err := json.Marshal(w)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, rc.BaseURL+"/register", bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("register status: %d", resp.StatusCode)
	}
	return nil
}

func (rc *RegistryClient) Heartbeat(id string, url string, load int) error {
	c := rc.HTTPClient
	if c == nil {
		c = &http.Client{}
	}
	payload := map[string]any{"id": id, "url": url, "load": load}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, rc.BaseURL+"/heartbeat", bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("heartbeat status: %d", resp.StatusCode)
	}
	return nil
}

func (rc *RegistryClient) Allocate(service string) (WorkerInfo, error) {
	c := rc.HTTPClient
	if c == nil {
		c = &http.Client{}
	}
	req, err := http.NewRequest(http.MethodGet, rc.BaseURL+"/allocate?service="+service, nil)
	if err != nil {
		return WorkerInfo{}, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return WorkerInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return WorkerInfo{}, fmt.Errorf("allocate status: %d", resp.StatusCode)
	}
	var w WorkerInfo
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&w); err != nil {
		return WorkerInfo{}, err
	}
	return w, nil
}

func (rc *RegistryClient) List(service string) ([]WorkerInfo, error) {
	c := rc.HTTPClient
	if c == nil {
		c = &http.Client{}
	}
	req, err := http.NewRequest(http.MethodGet, rc.BaseURL+"/list?service="+service, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list status: %d", resp.StatusCode)
	}
	var arr []WorkerInfo
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&arr); err != nil {
		return nil, err
	}
	return arr, nil
}
