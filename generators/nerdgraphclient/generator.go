package nerdgraphclient

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

func (g *Generator) Generate(s *schema.Schema, genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) error {
	if genConfig == nil {
		return fmt.Errorf("unable to Generate with nil genConfig")
	}

	if pkgConfig == nil {
		return fmt.Errorf("unable to Generate with nil pkgConfig")
	}

	expandedTypes, err := schema.ExpandTypes(s, pkgConfig.Types, pkgConfig.Methods)
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

	g.PackageName = pkgConfig.Name
	g.Imports = pkgConfig.Imports

	if structsForGen != nil {
		g.Types = *structsForGen
	}

	if enumsForGen != nil {
		g.Enums = *enumsForGen
	}

	if scalarsForGen != nil {
		g.Scalars = *scalarsForGen
	}

	if interfacesForGen != nil {
		g.Interfaces = *interfacesForGen
	}

	methodsForGen, err := lang.GenerateGoMethodsForPackage(s, genConfig, pkgConfig)
	if err != nil {
		return err
	}

	if methodsForGen != nil {
		g.Methods = *methodsForGen
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

	c := codegen.CodeGen{
		TemplateDir:     templateDir,
		TemplateName:    templateName,
		DestinationFile: filePath,
		DestinationDir:  destinationPath,
	}

	printSuccessMessage(pkgConfig, filePath)

	return c.WriteFile(g)
}

// printSuccessMessage prints a message to the terminal letting the user know
// that code generation was a success and outputs the package and file path for reference.
//
// Emoji unicode reference: http://www.unicode.org/emoji/charts/emoji-list.html
func printSuccessMessage(pkgConfig *config.PackageConfig, filePath string) {
	fmt.Print("\n\u2705 Code generation complete: \n\n")
	fmt.Printf("   Package:   %v \n", pkgConfig.Path)
	fmt.Printf("   File:      %v \n", filePath)
	fmt.Println("")
}
