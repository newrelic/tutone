package codegen

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"text/template"

	log "github.com/sirupsen/logrus"
	"golang.org/x/tools/imports"

	"github.com/newrelic/tutone/internal/output"
	"github.com/newrelic/tutone/internal/util"
	"github.com/newrelic/tutone/templates"
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
// Templates are resolved in order:
//  1. Local filesystem (templateDir/templateName) — allows per-repo overrides during development.
//  2. Embedded templates bundled in the tutone binary — used when running from a consuming repo
//     (e.g. newrelic-client-go, terraform-provider-newrelic) that has no local templates/ directory.
func (c *CodeGen) WriteFile(g Generator) error {
	var err error

	if _, err = os.Stat(c.DestinationDir); os.IsNotExist(err) {
		if err = os.Mkdir(c.DestinationDir, 0755); err != nil {
			return err
		}
	}

	templatePath := path.Join(c.TemplateDir, c.TemplateName)
	templateName := path.Base(templatePath)

	tmpl, err := resolveTemplate(templateName, templatePath)
	if err != nil {
		return err
	}

	// Create the destination file only after the template is successfully parsed,
	// so a parse failure never leaves a 0-byte file on disk.
	file, err := os.Create(c.DestinationFile)
	if err != nil {
		return err
	}
	defer file.Close()

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

	return nil
}

// resolveTemplate parses a template by name+path, trying the local filesystem first
// and falling back to the embedded templates bundled in the binary.
func resolveTemplate(templateName, templatePath string) (*template.Template, error) {
	funcs := util.GetTemplateFuncs()

	// 1. Try local filesystem — honours per-repo overrides.
	if _, err := os.Stat(templatePath); err == nil {
		return template.New(templateName).Funcs(funcs).ParseFiles(templatePath)
	}

	// 2. Fall back to embedded FS — keeps the binary self-contained.
	// The embed.FS in templates/tmpl.go is rooted at the templates/ directory,
	// so paths inside it are "terraform/resource.go.tmpl" (no "templates/" prefix).
	// Strip the leading "templates/" segment before looking up in the embedded FS.
	embeddedPath := strings.TrimPrefix(templatePath, "templates/")
	log.Debugf("template not found locally at %q — using embedded copy at %q", templatePath, embeddedPath)
	return template.New(templateName).Funcs(funcs).ParseFS(templates.FS, embeddedPath)
}

// resolveTemplateFromFS parses a template from an explicit fs.FS (used by tests).
func resolveTemplateFromFS(templateName, templatePath string, fsys fs.FS) (*template.Template, error) {
	return template.New(templateName).Funcs(util.GetTemplateFuncs()).ParseFS(fsys, templatePath)
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

	output.PrintSuccessMessage(c.DestinationDir, []string{c.DestinationFile})

	return nil
}
