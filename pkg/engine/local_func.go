package engine

import (
	"context"
	"fmt"
	"strings"
)

func UpperFunc(ctx context.Context, input interface{}, params map[string]interface{}) (interface{}, error) {
	if s, ok := input.(string); ok {
		return strings.ToUpper(s), nil
	}

	if s, ok := params["text"].(string); ok {
		return strings.ToUpper(s), nil
	}

	return nil, fmt.Errorf("expected string input")
}

func LogResultFunc(ctx context.Context, input interface{}, params map[string]interface{}) (interface{}, error) {
	fmt.Printf("LOG RESULT: %v\n", input)
	return input, nil
}

func MulFunc(ctx context.Context, input interface{}, params map[string]interface{}) (interface{}, error) {
	f := 0.0
	if v, ok := input.(float64); ok {
		f = v
	}
	m := 1.0
	if mv, ok := params["mul"].(float64); ok {
		m = mv
	}
	return f * m, nil
}
