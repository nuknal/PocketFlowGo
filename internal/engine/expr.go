package engine

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// evalExpr evaluates a boolean expression against the given context (shared, params, input).
// It supports logic operators (and, or, not) and comparison operators (eq, ne, gt, lt, etc.).
func evalExpr(expr map[string]interface{}, shared map[string]interface{}, params map[string]interface{}, input interface{}) bool {
	if expr == nil {
		return false
	}
	for k, v := range expr {
		switch k {
		case "and":
			if arr, ok := v.([]interface{}); ok {
				for _, it := range arr {
					if !evalExpr(toMap(it), shared, params, input) {
						return false
					}
				}
				return true
			}
		case "or":
			if arr, ok := v.([]interface{}); ok {
				for _, it := range arr {
					if evalExpr(toMap(it), shared, params, input) {
						return true
					}
				}
				return false
			}
		case "not":
			return !evalExpr(toMap(v), shared, params, input)
		case "eq", "ne", "gt", "lt", "ge", "le":
			if arr, ok := v.([]interface{}); ok && len(arr) == 2 {
				a := resolveVal(arr[0], shared, params, input)
				b := resolveVal(arr[1], shared, params, input)
				switch k {
				case "eq":
					return equal(a, b)
				case "ne":
					return !equal(a, b)
				case "gt":
					return cmp(a, b) > 0
				case "lt":
					return cmp(a, b) < 0
				case "ge":
					return cmp(a, b) >= 0
				case "le":
					return cmp(a, b) <= 0
				}
			}
		case "exists":
			if s, ok := v.(string); ok {
				val := resolveRef(s, shared, params, input)
				return val != nil
			}
		case "in":
			if arr, ok := v.([]interface{}); ok && len(arr) == 2 {
				val := resolveVal(arr[0], shared, params, input)
				col := resolveVal(arr[1], shared, params, input)
				switch c := col.(type) {
				case []interface{}:
					for _, x := range c {
						if equal(x, val) {
							return true
						}
					}
				case string:
					if s, ok := val.(string); ok {
						return strings.Contains(c, s)
					}
				}
				return false
			}
		case "contains":
			if arr, ok := v.([]interface{}); ok && len(arr) == 2 {
				col := resolveVal(arr[0], shared, params, input)
				val := resolveVal(arr[1], shared, params, input)
				switch c := col.(type) {
				case []interface{}:
					for _, x := range c {
						if equal(x, val) {
							return true
						}
					}
				case string:
					if s, ok := val.(string); ok {
						return strings.Contains(c, s)
					}
				}
				return false
			}
		}
	}
	return false
}

// toMap helper converts interface{} to map[string]interface{}.
func toMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{"eq": []interface{}{v, true}}
}

// resolveVal resolves a value or a reference path (e.g., "$input.x").
func resolveVal(v interface{}, shared map[string]interface{}, params map[string]interface{}, input interface{}) interface{} {
	if s, ok := v.(string); ok {
		return resolveRef(s, shared, params, input)
	}
	return v
}

// resolveRef resolves a variable reference path from params, shared state, or input.
func resolveRef(path string, shared map[string]interface{}, params map[string]interface{}, input interface{}) interface{} {
	if strings.HasPrefix(path, "$params.") {
		k := strings.TrimPrefix(path, "$params.")
		if k != "" && k[0] == '.' {
			k = k[1:]
		}
		return getByPath(params, k)
	}
	if strings.HasPrefix(path, "$shared.") {
		k := strings.TrimPrefix(path, "$shared.")
		if k != "" && k[0] == '.' {
			k = k[1:]
		}
		return getByPath(shared, k)
	}
	if strings.HasPrefix(path, "$input") {
		p := strings.TrimPrefix(path, "$input")
		if p == "" {
			return input
		}
		if p[0] == '.' {
			p = p[1:]
		}
		return getByPath(input, p)
	}
	return path
}

func asFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case string:
		if f, err := strconv.ParseFloat(x, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// cmp compares two values (a and b) and returns -1 if a < b, 1 if a > b, 0 if equal.
func cmp(a interface{}, b interface{}) int {
	if fa, ok := asFloat(a); ok {
		if fb, ok := asFloat(b); ok {
			if fa < fb {
				return -1
			}
			if fa > fb {
				return 1
			}
			return 0
		}
	}
	sa := toString(a)
	sb := toString(b)
	if sa < sb {
		return -1
	}
	if sa > sb {
		return 1
	}
	return 0
}

func toString(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return fmt.Sprintf("%g", x)
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

func equal(a interface{}, b interface{}) bool {
	if fa, ok := asFloat(a); ok {
		if fb, ok := asFloat(b); ok {
			return fa == fb
		}
	}
	return toString(a) == toString(b)
}
