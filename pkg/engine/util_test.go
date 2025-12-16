package engine

import (
	"reflect"
	"testing"
)

func TestGetByPath(t *testing.T) {
	cases := []struct {
		name string
		v    interface{}
		path string
		want interface{}
	}{
		{"empty_path_map", map[string]interface{}{"a": 1.0}, "", map[string]interface{}{"a": 1.0}},
		{"nested_map", map[string]interface{}{"a": map[string]interface{}{"b": 3.0}}, "a.b", 3.0},
		{"array_index", map[string]interface{}{"a": []interface{}{1.0, 2.0, 3.0}}, "a[1]", 2.0},
		{"array_index_out_of_bounds", map[string]interface{}{"a": []interface{}{1.0}}, "a[5]", nil},
		{"missing_key", map[string]interface{}{"a": 1.0}, "b", nil},
		{"type_mismatch_after_key", map[string]interface{}{"a": 1.0}, "a.b", nil},
		{"array_then_map", map[string]interface{}{"a": []interface{}{map[string]interface{}{"x": 9.0}}}, "a[0].x", 9.0},
		{"skip_empty_segments", map[string]interface{}{"a": map[string]interface{}{"b": 7.0}}, "a..b", 7.0},
		{"array_root_empty_path", []interface{}{1.0, 2.0}, "", []interface{}{1.0, 2.0}},
		{"array_root_segment_not_supported", []interface{}{[]interface{}{1.0}}, "[0]", nil},
		{"negative_index", map[string]interface{}{"a": []interface{}{1.0}}, "a[-1]", nil},
		{"invalid_index_text", map[string]interface{}{"a": []interface{}{1.0}}, "a[x]", nil},
		{"invalid_empty_index", map[string]interface{}{"a": []interface{}{1.0}}, "a[]", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := getByPath(c.v, c.path)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got=%v want=%v", got, c.want)
			}
		})
	}
}
