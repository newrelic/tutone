package typegen

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/codegen"
	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
	"github.com/newrelic/tutone/pkg/lang"
)

type Generator struct {
	lang.GolangGenerator
}

// Generate is the entry point for this Generator.
func (g *Generator) Generate(s *schema.Schema, genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) error {
	if genConfig == nil {
		return fmt.Errorf("unable to Generate with nil genConfig")
	}

	if pkgConfig == nil {
		return fmt.Errorf("unable to Generate with nil pkgConfig")
	}

	expandedTypes, err := schema.ExpandTypes(s, pkgConfig)
	if err != nil {
		log.Error(err)
	}

	structsForGen, enumsForGen, scalarsForGen, interfacesForGen, err := lang.GenerateGoTypesForPackage(s, genConfig, pkgConfig, expandedTypes)
	if err != nil {
		return err
	}

	// The Execute() below expects to have Generator g populated for use in the template files.
	g.GolangGenerator.SetPackageNaming(pkgConfig)
	g.GolangGenerator.Imports = pkgConfig.Imports
	g.GolangGenerator.KeepExisting = lang.KeptNames(pkgConfig, expandedTypes)

	if structsForGen != nil {
		g.GolangGenerator.Types = *structsForGen
	}

	if enumsForGen != nil {
		g.GolangGenerator.Enums = *enumsForGen
	}

	if scalarsForGen != nil {
		g.GolangGenerator.Scalars = *scalarsForGen
	}

	if interfacesForGen != nil {
		g.GolangGenerator.Interfaces = *interfacesForGen
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

	// Default file name is 'types.go'
	fileName := "types.go"
	if genConfig.FileName != "" {
		fileName = genConfig.FileName
	}

	// The file is created by WriteFile, after it has read what is already there.
	// Creating it here would truncate hand-written code before it could be kept.
	filePath := fmt.Sprintf("%s/%s", destinationPath, fileName)

	templateName := "types.go.tmpl"
	if genConfig.TemplateName != "" {
		templateName = genConfig.TemplateName
	}

	templateDir := "templates/typegen"
	if genConfig.TemplateDir != "" {
		templateDir = genConfig.TemplateDir
	}

	c := codegen.CodeGen{
		TemplateDir:     templateDir,
		TemplateName:    templateName,
		DestinationFile: filePath,
		DestinationDir:  destinationPath,

		// Only an explicit go_package_name overrides what the template writes.
		GoPackageName: pkgConfig.GoPackageName,

		PreserveHandWritten: pkgConfig.GetPreserveHandWritten(),
		KeepNames:           g.KeepExisting,
	}

	return c.WriteFile(g)
}
