package codegen

import (
	"bytes"
	"go/format"
	"html/template"
	"os"
	"path"

	"github.com/Masterminds/sprig"
)

type CodeGen struct {
	TemplateDir     string
	TemplateName    string
	DestinationDir  string
	DestinationFile string
	Source          Path
	Destination     Path
}

type Path struct {
	// Directory is the path to directory that will store the file, eg: pkg/nerdgraph
	Directory string
	// File is the name of the file within the directory
	File string
}

// WriteFile creates a new file, where the output from rendering template using the received Generator will be stored.
func (c *CodeGen) WriteFile(g Generator) error {
	var err error

	if _, err = os.Stat(c.DestinationDir); os.IsNotExist(err) {
		if err = os.Mkdir(c.DestinationDir, 0755); err != nil {
			return err
		}
	}

	file, err := os.Create(c.DestinationFile)
	if err != nil {
		return err
	}

	defer file.Close()

	templatePath := path.Join(c.TemplateDir, c.TemplateName)
	templateName := path.Base(templatePath)

	tmpl, err := template.New(templateName).Funcs(sprig.FuncMap()).ParseFiles(templatePath)
	// tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return err
	}

	var resultBuf bytes.Buffer

	err = tmpl.Execute(&resultBuf, g)
	if err != nil {
		return err
	}

	formatted, err := format.Source(resultBuf.Bytes())
	if err != nil {
		return err
	}

	_, err = file.WriteAt(formatted, 0)
	if err != nil {
		return err
	}

	return nil
}
