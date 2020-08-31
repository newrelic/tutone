package util

import "encoding/json"

func ToJSONPretty(data interface{}) string {
	c, _ := json.MarshalIndent(data, "", "  ")

	return string(c)
}
