package nerdgraphclient

import (
	"fmt"
	"os"
	"strings"

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
	var generatedFiles []string

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

		// Only explicit go_package_name and client_type_name override what the template writes.
		GoPackageName:   pkgConfig.GoPackageName,
		ReceiverRenames: clientTypeRenames(pkgConfig),
		TypeStandIns:    typeStandIns(g.Mutations),
		OmitUnset:       omitUnset(g.Queries, g.Mutations),

		PreserveHandWritten: pkgConfig.GetPreserveHandWritten(),
		KeepNames:           g.KeepExisting,
	}

	err = c.WriteFile(g)
	if err != nil {
		return err
	}

	generatedFiles = []string{filePath}

	if pkgConfig.IncludeIntegrationTest {
		testFileName := fmt.Sprintf("%s_integration_test.go", strings.ToLower(pkgConfig.GetFileName()))
		testFilePath := fmt.Sprintf("%s/%s", destinationPath, testFileName)

		// Only create an integration test file if it doesn't exist. We don't want to overwrite
		// existing integration test files when regenerating package code because integration tests
		// are only scaffolded at time of generation and then manually edited.
		if !fileExists(testFilePath) {
			cg := codegen.CodeGen{
				TemplateDir:     templateDir,
				TemplateName:    "integration_test.go.tmpl",
				DestinationFile: testFilePath,
				DestinationDir:  destinationPath,
				GoPackageName:   pkgConfig.GoPackageName,
			}

			err = cg.WriteFile(g)
			if err != nil {
				log.Warnf("Error generating integration test file.")
			}

			generatedFiles = append(generatedFiles, testFilePath)
		}
	}

	output.PrintSuccessMessage(c.DestinationDir, generatedFiles)

	return nil
}

// clientTypeRenames maps the receiver type a template that predates client_type_name
// writes to the configured one. It is nil when the two are the same.
func clientTypeRenames(pkgConfig *config.PackageConfig) map[string]string {
	from, to := pkgConfig.DefaultClientTypeName(), pkgConfig.GetClientTypeName()
	if from == to {
		return nil
	}

	return map[string]string{from: to}
}

// typeStandIns lists the identifiers the templates print for the return types of the
// mutations that have a return alias, and the types and field name to put there.
func typeStandIns(mutations []lang.GoMethod) []codegen.TypeStandIn {
	var standIns []codegen.TypeStandIn

	for _, m := range mutations {
		if m.ReturnAlias != "" {
			standIns = append(standIns, codegen.TypeStandIn{
				Ident: m.ReturnAlias,
				Type:  m.ReturnType,
				Name:  lang.MutationResultField,
			})
		}
	}

	return standIns
}

// omitUnset lists, for the methods generated with include_optional_arguments, the
// functions the templates print for them and the parameters that are left out of the
// request when nil. A query method has the name the config gives it and, for templates
// that print one, a context variant. A mutation has its name with the first letter
// capitalized, and the same variant.
func omitUnset(queries, mutations []lang.GoMethod) []codegen.UnsetArguments {
	var out []codegen.UnsetArguments

	add := func(plain, withContext string, m lang.GoMethod) {
		if len(m.UnsetParams) == 0 {
			return
		}

		u := codegen.UnsetArguments{Funcs: []string{plain, withContext}}
		for _, p := range m.UnsetParams {
			u.Params = append(u.Params, codegen.UnsetParam{Param: p.Param, Key: p.Key})
		}

		out = append(out, u)
	}

	for _, m := range queries {
		add(m.MethodName, m.ContextMethodName, m)
	}

	for _, m := range mutations {
		name := lang.UpperFirst(m.Name)
		add(name, name+"WithContext", m)
	}

	return out
}

func fileExists(filePath string) bool {
	_, error := os.Stat(filePath)
	return !os.IsNotExist(error)
}
