package nerdgraphclient

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/codegen"
	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/output"
	"github.com/newrelic/tutone/internal/schema"
	"github.com/newrelic/tutone/pkg/lang"
)

type Generator struct {
	lang.GolangGenerator
}

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

	// TODO idea:
	// err = lang.GenerateTypesForPackage(&g)
	// if err != nil {
	// 	return err
	// }

	// lang.Normalize(&g, genConfig, pkgConfig)

	g.GolangGenerator.PackageName = pkgConfig.Name
	g.GolangGenerator.Imports = pkgConfig.Imports

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

	mutationsForGen, err := lang.GenerateGoMethodMutationsForPackage(s, genConfig, pkgConfig)
	if err != nil {
		return err
	}

	if mutationsForGen != nil {
		g.GolangGenerator.Mutations = *mutationsForGen
	}

	queriesForGen, err := lang.GenerateGoMethodQueriesForPackage(s, genConfig, pkgConfig)
	if err != nil {
		return err
	}

	if queriesForGen != nil {
		g.GolangGenerator.Queries = *queriesForGen
	}

	return nil
}

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

	// Default file name is 'nerdgraph.go'
	fileName := "nerdgraphclient.go"
	if genConfig.FileName != "" {
		fileName = genConfig.FileName
		if err != nil {
			return err
		}
	}

	templateName := "client.go.tmpl"
	if genConfig.TemplateName != "" {
		templateName = genConfig.TemplateName
		if err != nil {
			return err
		}
	}

	filePath, err := codegen.RenderStringFromGenerator(fmt.Sprintf("%s/%s", destinationPath, fileName), g)
	if err != nil {
		return err
	}

	templateDir := "templates/nerdgraphclient"
	if genConfig.TemplateDir != "" {
		templateDir, err = codegen.RenderStringFromGenerator(genConfig.TemplateDir, g)
		if err != nil {
			return err
		}
	}

	// fileName_ := fmt.Sprintf("%s_integration_test.go", strings.ToLower(pkgConfig.Name))
	// testFilePath, err := codegen.RenderStringFromGenerator(fmt.Sprintf("%s/%s", destinationPath, fileName_), g)
	// if err != nil {
	// 	return err
	// }

	// cg := codegen.CodeGen{
	// 	TemplateDir:     templateDir,
	// 	TemplateName:    "integration_test.go.tmpl",
	// 	DestinationFile: testFilePath,
	// 	DestinationDir:  destinationPath,
	// }

	// err = cg.WriteFile(g)
	// if err != nil {
	// 	return err
	// }

	c := codegen.CodeGen{
		TemplateDir:     templateDir,
		TemplateName:    templateName,
		DestinationFile: filePath,
		DestinationDir:  destinationPath,
	}

	err = c.WriteFile(g)
	if err != nil {
		return err
	}

	output.PrintSuccessMessage(c.DestinationDir, []string{filePath})

	return nil
}
