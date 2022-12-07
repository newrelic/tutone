package terraform

import (
	"errors"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/codegen"
	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
)

type Generator struct {
	TerraformResourceGenerator
}

type TerraformResourceGenerator struct {
	PackageName string
	Imports     []string
	Resources   []Resource
}

type Resource struct {
	Name       string
	FileName   string
	Attributes []TerraformSchemaAttribute
}

type TerraformSchemaAttribute struct {
	Key  string
	Type string
}

func (g *Generator) Generate(s *schema.Schema, genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) error {
	fmt.Print("\n****************************\n")
	fmt.Printf("\n TF Generate:  %+v \n", genConfig)
	fmt.Print("\n****************************\n")

	if genConfig == nil {
		return fmt.Errorf("unable to Generate with nil genConfig")
	}

	if pkgConfig == nil {
		return fmt.Errorf("unable to Generate with nil pkgConfig")
	}

	// The Execute() below expects to have Generator g populated for use in the template files.
	g.TerraformResourceGenerator.PackageName = pkgConfig.Name
	g.TerraformResourceGenerator.Imports = pkgConfig.Imports

	for _, r := range pkgConfig.Resources {
		g.TerraformResourceGenerator.Resources = append(g.TerraformResourceGenerator.Resources, Resource{
			Name:     r.Name,
			FileName: r.FileName,
		})
	}
	return nil
}

// Execute performs the template render and file writement, according to the received configurations for the current Generator instance.
func (g *Generator) Execute(genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) error {
	var err error

	// Default to project root for types
	destinationPath := "./"
	if pkgConfig.Path != "" {
		destinationPath = pkgConfig.Path
	}

	if _, err = os.Stat(destinationPath); os.IsNotExist(err) {
		if err = os.Mkdir(destinationPath, 0755); err != nil {
			log.Error(err)
		}
	}

	for _, resource := range g.TerraformResourceGenerator.Resources {
		if resource.FileName == "" {
			return errors.New("resource file name is required")
		}

		fileName := resource.FileName
		if genConfig.FileName != "" {
			fileName = genConfig.FileName
		}

		filePath := fmt.Sprintf("%s/%s", destinationPath, fileName)
		file, createFileErr := os.Create(filePath)
		if createFileErr != nil {
			log.Error(createFileErr)
		}
		defer file.Close()

		templateName := "resource.go.tmpl"
		if genConfig.TemplateName != "" {
			templateName = genConfig.TemplateName
		}

		templateDir := "templates/terraform"
		if genConfig.TemplateDir != "" {
			templateDir = genConfig.TemplateDir
		}

		c := codegen.CodeGen{
			TemplateDir:     templateDir,
			TemplateName:    templateName,
			DestinationFile: filePath,
			DestinationDir:  destinationPath,
		}

		err = c.WriteFile(g)
	}

	return err
}
