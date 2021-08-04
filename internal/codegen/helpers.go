package codegen

import (
	"bytes"
	"text/template"

	"github.com/newrelic/tutone/internal/util"
)

// RenderStringFromGenerator receives a Generator that is used to render the received template string.
func RenderStringFromGenerator(s string, g Generator) (string, error) {
	tmpl, err := template.New("string").Funcs(util.GetTemplateFuncs()).Parse(s)
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

// RenderTemplate parses and returns the rendered string of the provided template.
// The template is also assigned the provided name for reference.
//
// TODO: How can we compose a template of embedded templates to help scale?
//       Templates are stored as map[string]*Template - ("someName": *Template).
//       https://stackoverflow.com/questions/41176355/go-template-name
func RenderTemplate(templateName string, templateString string, data interface{}) (string, error) {
	tmpl, err := template.New(templateName).Funcs(util.GetTemplateFuncs()).Parse(templateString)
	if err != nil {
		return "", err
	}

	var buffer bytes.Buffer
	err = tmpl.Execute(&buffer, data)
	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}
