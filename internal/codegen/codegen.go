package codegen

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"text/template"

	log "github.com/sirupsen/logrus"
	"golang.org/x/tools/imports"

	"github.com/newrelic/tutone/internal/output"
	"github.com/newrelic/tutone/internal/util"
	"github.com/newrelic/tutone/pkg/lang"
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

	tmpl, err := template.New(templateName).Funcs(util.GetTemplateFuncs()).ParseFiles(templatePath)
	if err != nil {
		return err
	}

	var resultBuf bytes.Buffer

	err = tmpl.Execute(&resultBuf, g)
	if err != nil {
		return err
	}

	formatted, err := imports.Process(file.Name(), resultBuf.Bytes(), nil)
	if err != nil {
		log.Error(resultBuf.String())

		_, err = file.WriteAt(resultBuf.Bytes(), 0)
		if err != nil {
			log.Error(err)
		}
	}

	_, err = file.WriteAt(formatted, 0)
	if err != nil {
		return err
	}

	output.PrintSuccessMessage(c.DestinationDir, c.DestinationFile)

	return nil
}

func (c *CodeGen) WriteFileFromTemplateString(g Generator, templateString string) error {
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

	tmpl, err := template.New(templateName).Funcs(util.GetTemplateFuncs()).Parse(templateString)
	if err != nil {
		return err
	}

	var resultBuf bytes.Buffer

	err = tmpl.Execute(&resultBuf, g)
	if err != nil {
		return err
	}

	formatted, err := imports.Process(file.Name(), resultBuf.Bytes(), nil)
	if err != nil {
		log.Error(resultBuf.String())
		return fmt.Errorf("failed to format buffer: %s", err)
	}

	_, err = file.WriteAt(formatted, 0)
	if err != nil {
		return err
	}

	output.PrintSuccessMessage(c.DestinationDir, c.DestinationFile)

	return nil
}

func (c *CodeGen) RenderTerraformTemplate(g lang.TerraformResourceTemplateData) error {
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

	tmpl, err := template.New(templateName).Funcs(util.GetTemplateFuncs()).ParseFiles(templatePath)
	if err != nil {
		return err
	}

	var resultBuf bytes.Buffer

	err = tmpl.Execute(&resultBuf, g)
	if err != nil {
		return err
	}

	formatted, err := imports.Process(file.Name(), resultBuf.Bytes(), nil)
	if err != nil {
		log.Error(resultBuf.String())

		_, err = file.WriteAt(resultBuf.Bytes(), 0)
		if err != nil {
			log.Error(err)
		}
	}

	_, err = file.WriteAt(formatted, 0)
	if err != nil {
		return err
	}

	output.PrintSuccessMessage(c.DestinationDir, c.DestinationFile)

	return nil
}
