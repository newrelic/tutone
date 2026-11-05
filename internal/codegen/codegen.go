package codegen

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	log "github.com/sirupsen/logrus"
	"golang.org/x/tools/imports"

	"github.com/newrelic/tutone/internal/output"
	"github.com/newrelic/tutone/internal/util"
)

type CodeGen struct {
	TemplateDir     string
	TemplateName    string
	DestinationDir  string
	DestinationFile string
	Source          Path
	Destination     Path

	// GoPackageName, when set, is the name the rendered file's package clause gets
	// if the template wrote another. Templates written before go_package_name
	// existed print the lower-cased config name.
	GoPackageName string
	// ReceiverRenames maps the type a template gave its methods' receivers to the
	// type they should have, for the same reason (client_type_name).
	ReceiverRenames map[string]string

	// PreserveHandWritten keeps, when the destination already exists, the
	// declarations in it that the generator does not produce or that are marked
	// DO NOT DELETE or DO NOT REGENERATE.
	PreserveHandWritten bool
	// KeepNames names declarations of the existing file that are kept as they are,
	// as if marked: for each name the type, const, var or function of that name and
	// the methods of a type of that name. It applies only with PreserveHandWritten.
	KeepNames []string
}

type Path struct {
	// Directory is the path to directory that will store the file, eg: pkg/nerdgraph
	Directory string
	// File is the name of the file within the directory
	File string
}

// WriteFile renders the template using the received Generator and stores the result in the destination file.
//
// The package clause and method receivers are set to the configured names before
// the output is formatted, so any template honours them. The destination is not
// touched until the output is rendered, so the previous version can be read first.
// When PreserveHandWritten is set, hand-written declarations in that previous
// version are carried into the new file (see MergeHandWritten).
func (c *CodeGen) WriteFile(g Generator) error {
	var err error

	if _, err = os.Stat(c.DestinationDir); os.IsNotExist(err) {
		if err = os.Mkdir(c.DestinationDir, 0755); err != nil {
			return err
		}
	}

	dest := ResolveCaseSpelling(c.DestinationFile)

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

	formatted, err := imports.Process(dest, c.applyNaming(resultBuf.Bytes()), nil)
	if err != nil {
		log.Error(resultBuf.String())

		// Keep the raw output so it can be inspected, as before.
		return os.WriteFile(dest, resultBuf.Bytes(), 0644)
	}

	if c.PreserveHandWritten {
		formatted, err = preserveHandWritten(dest, formatted, c.KeepNames)
		if err != nil {
			return err
		}
	}

	return os.WriteFile(dest, formatted, 0644)
}

// applyNaming sets the package name and receiver types the config asks for on a
// template's output. Source that does not parse is returned as it is: formatting
// fails on it next, and that is reported there.
func (c *CodeGen) applyNaming(rendered []byte) []byte {
	out := rendered

	if c.GoPackageName != "" {
		if renamed, err := SetPackageName(out, c.GoPackageName); err == nil {
			out = renamed
		}
	}

	if len(c.ReceiverRenames) > 0 {
		if renamed, err := RenameReceivers(out, c.ReceiverRenames); err == nil {
			out = renamed
		}
	}

	return out
}

// preserveHandWritten merges the declarations kept from the existing file at
// dest, if there is one, into the freshly generated source.
func preserveHandWritten(dest string, generated []byte, keepNames []string) ([]byte, error) {
	existing, err := os.ReadFile(dest)
	if os.IsNotExist(err) {
		return generated, nil
	}

	if err != nil {
		return nil, err
	}

	merged, kept, err := MergeHandWritten(generated, existing, keepNames...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", dest, err)
	}

	if len(kept) == 0 {
		return generated, nil
	}

	log.WithFields(log.Fields{
		"file":  dest,
		"count": len(kept),
		"names": kept,
	}).Info("kept hand-written declarations")

	formatted, err := imports.Process(dest, merged, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: formatting after keeping hand-written declarations: %w", dest, err)
	}

	return formatted, nil
}

// ResolveCaseSpelling returns the path of the file that already exists in the
// directory under a name that differs from file's only in letter case, or file
// itself if there is none.
//
// Without this, a name such as agentApplications_api.go next to the existing
// agentapplications_api.go is one file on a case-insensitive file system (macOS)
// and two on Linux, where Go then refuses to build the package. Writing to the
// spelling that is already there behaves the same on both, and leaves the
// repository with the one file it had.
func ResolveCaseSpelling(file string) string {
	dir, base := filepath.Split(file)
	if dir == "" {
		dir = "."
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return file
	}

	for _, e := range entries {
		if e.Name() == base {
			return file
		}
	}

	for _, e := range entries {
		if strings.EqualFold(e.Name(), base) {
			log.WithFields(log.Fields{
				"generated_name": base,
				"existing_name":  e.Name(),
			}).Warn("writing to the existing file name, which differs only in letter case; set file_name on the package to silence this")

			return filepath.Join(filepath.Dir(file), e.Name())
		}
	}

	return file
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
