package util

import (
	"reflect"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

func GetTemplateFuncs() template.FuncMap {
	funcs := sprig.TxtFuncMap()

	// Custom funcs
	funcs["hasField"] = hasField

	return funcs
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
