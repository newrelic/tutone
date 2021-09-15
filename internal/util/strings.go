package util

import (
	"regexp"
	"strings"
)

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

// ToSnakeCase returns the string formatted in Snake Case
func ToSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

// StringInStrings checks if the string is in the slice of strings
func StringInStrings(s string, ss []string) bool {
	for _, sss := range ss {
		if s == sss {
			return true
		}
	}

	return false
}
