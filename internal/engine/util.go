package engine

import (
	"encoding/json"
	"strconv"
	"strings"
)

// toJSON marshals a value to a JSON string, ignoring errors.
func toJSON(v interface{}) string { b, _ := json.Marshal(v); return string(b) }

func ternary[T any](cond bool, a T, b T) T {
	if cond {
		return a
	}
	return b
}

type errorString string

func (e errorString) Error() string { return string(e) }

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// pickAction extracts an action string from a result map based on a key.
func pickAction(res interface{}, key string) string {
	if m, ok := res.(map[string]interface{}); ok {
		if v, ok := m[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

// findNext determines the next node key based on the current node and action.
func findNext(edges []DefEdge, from string, action string) string {
	a := action
	if a == "" {
		a = "default"
	}
	for _, ed := range edges {
		if ed.From == from && ed.Action == a {
			return ed.To
		}
	}
	return ""
}

func indexKey(i int) string { return strconv.Itoa(i) }

// getByPath retrieves a value from a nested map/slice structure using dot notation (e.g. "a.b[0].c").
func getByPath(v interface{}, path string) interface{} {
	if path == "" {
		return v
	}
	parts := strings.Split(path, ".")
	cur := v
	for _, seg := range parts {
		if seg == "" {
			continue
		}
		name, idx, hasIdx := parseSegment(seg)
		switch m := cur.(type) {
		case map[string]interface{}:
			cur = m[name]
		default:
			return nil
		}
		if hasIdx {
			switch arr := cur.(type) {
			case []interface{}:
				if idx >= 0 && idx < len(arr) {
					cur = arr[idx]
				} else {
					return nil
				}
			default:
				return nil
			}
		}
		if cur == nil {
			return nil
		}
	}
	return cur
}

// parseSegment parses a path segment like "items[0]" into name="items", idx=0, hasIdx=true.
func parseSegment(seg string) (string, int, bool) {
	i := strings.Index(seg, "[")
	if i < 0 {
		return seg, -1, false
	}
	j := strings.Index(seg, "]")
	if j < 0 || j < i+1 {
		return seg, -1, false
	}
	name := seg[:i]
	numStr := seg[i+1 : j]
	n, err := strconv.Atoi(numStr)
	if err != nil {
		return seg, -1, false
	}
	return name, n, true
}
