package util

import (
	"reflect"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

func GetTemplateFuncs() template.FuncMap {
	funcs := sprig.TxtFuncMap()

	funcs["hasField"] = hasField
	funcs["snakeToCamelExport"] = SnakeToCamelExport
	funcs["join"] = strings.Join

	return funcs
}

// SnakeToCamelExport converts "alert_muting_rule" → "AlertMutingRule"
func SnakeToCamelExport(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return b.String()
}

func hasField(v interface{}, name string) bool {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return false
	}
	return rv.FieldByName(name).IsValid()
}
