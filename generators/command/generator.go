package command

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/codegen"
	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/filesystem"
	"github.com/newrelic/tutone/internal/schema"
	"github.com/newrelic/tutone/pkg/lang"
)

type Generator struct {
	lang.CommandGenerator
}

func (g *Generator) Generate(s *schema.Schema, genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) error {
	log.Debugf("Generate...")

	g.PackageName = pkgConfig.Name
	g.Imports = pkgConfig.Imports

	cmds := make([]lang.Command, len(pkgConfig.Commands))
	for i, c := range pkgConfig.Commands {
		cmds[i] = hydrateCommand(s, c, pkgConfig)
	}

	g.Commands = cmds

	return nil
}

func (g *Generator) Execute(genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) error {
	log.Debugf("Generating commands...")

	var templateStr string
	var err error

	destinationPath := pkgConfig.GetDestinationPath()
	if err = filesystem.MakeDir(destinationPath, 0775); err != nil {
		return err
	}

	hasTemplateURL := genConfig.TemplateURL != ""
	hasTemplateDir := genConfig.TemplateDir != ""

	if hasTemplateURL {
		templateStr, err = fetchRemoteTemplate(genConfig.TemplateURL)
		if err != nil {
			return err
		}
	}

	for _, command := range pkgConfig.Commands {
		fileName := "command.go"

		if command.FileName != "" {
			fileName = command.FileName
		}

		// Default template name is '{{ packageName }}.go.tmpl'
		templateName := "command.go.tmpl"
		if genConfig.TemplateName != "" {
			templateName = genConfig.TemplateName
		}

		fPath := fmt.Sprintf("%s/%s", destinationPath, fileName)
		destinationFile, err := codegen.RenderStringFromGenerator(fPath, g)
		if err != nil {
			return err
		}

		if hasTemplateURL && hasTemplateDir {
			return fmt.Errorf("generator configuration error: please set `templateDir` or `templateURL`, but not both")
		}

		templateDir := "templates/command"
		if hasTemplateDir {
			templateDir, err = codegen.RenderStringFromGenerator(genConfig.TemplateDir, g)
			if err != nil {
				return err
			}
		}

		c := codegen.CodeGen{
			TemplateDir:     templateDir,
			TemplateName:    templateName,
			DestinationFile: destinationFile,
			DestinationDir:  destinationPath,
		}

		if templateStr != "" {
			if err := c.WriteFileFromTemplateString(g, templateStr); err != nil {
				return err
			}
		} else {
			if err := c.WriteFile(g); err != nil {
				return err
			}
		}
	}

	return nil
}
