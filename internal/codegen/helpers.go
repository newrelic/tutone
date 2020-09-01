package codegen

import (
	"bytes"
	"html/template"

	"github.com/Masterminds/sprig/v3"
)

// RenderStringFromGenerator receives a Generator that is used to render the received template string.
func RenderStringFromGenerator(s string, g Generator) (string, error) {
	tmpl, err := template.New("string").Funcs(sprig.FuncMap()).Parse(s)
	if err != nil {
		return "", err
	}

	var resultBuf bytes.Buffer

	err = tmpl.Execute(&resultBuf, g)
	if err != nil {
		return "", err
	}

	return resultBuf.String(), nil
}
