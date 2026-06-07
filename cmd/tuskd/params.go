package main

import "fmt"

func reqParams(req map[string]interface{}) (map[string]interface{}, bool) {
	raw, ok := req["params"]
	if !ok || raw == nil {
		return map[string]interface{}{}, true
	}
	params, ok := raw.(map[string]interface{})
	if !ok {
		return nil, false
	}
	return params, true
}

func asStringSlice(value interface{}) ([]string, error) {
	if value == nil {
		return nil, nil
	}

	switch v := value.(type) {
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("value is not a string")
			}
			result = append(result, s)
		}
		return result, nil
	case []string:
		return append([]string(nil), v...), nil
	default:
		return nil, fmt.Errorf("value is not an array")
	}
}

func getStringOrDefault(value interface{}) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}
