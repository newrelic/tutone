package terraform

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

// Generator implements codegen.Generator for the Terraform provider.
type Generator struct {
	lang.TerraformGenerator
}

// Generate populates the TerraformGenerator state from the schema and package config.
// Returns nil (no error) when pkgConfig.Terraform is nil — the generator is simply skipped.
func (g *Generator) Generate(s *schema.Schema, genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) error {
	if pkgConfig == nil {
		return fmt.Errorf("terraform generator: nil pkgConfig")
	}
	if pkgConfig.Terraform == nil {
		return nil
	}

	tfCfg := pkgConfig.Terraform

	g.TerraformConfig = tfCfg
	g.PackageName = pkgConfig.Name
	g.ImportPath = pkgConfig.ImportPath
	g.ResourceName = tfCfg.ResourceName
	g.ResourceTypeName = lang.SnakeToPascal(tfCfg.ResourceName)
	g.FuncPrefix = "resourceNewRelic" + g.ResourceTypeName

	// Derive or use explicit ClientService
	if tfCfg.ClientService != "" {
		g.ClientService = tfCfg.ClientService
	} else {
		g.ClientService = lang.DeriveClientService(pkgConfig.ImportPath)
		log.Warnf("terraform generator: client_service not set for %q, defaulting to %q — override if wrong",
			pkgConfig.Name, g.ClientService)
	}

	// Resolve Create mutation — explicit config takes precedence, then infer from mutations list
	createMutationName := tfCfg.CreateMutation
	if createMutationName == "" {
		createMutationName = findMutationByVerb(pkgConfig.Mutations, "create")
	}
	if createMutationName != "" {
		g.CreateMethod = lang.DeriveMethodName(createMutationName)
		createMutation, err := s.LookupMutationByName(createMutationName)
		if err != nil {
			return fmt.Errorf("terraform generator: create mutation %q not found: %w", createMutationName, err)
		}
		g.CreateInputType = lang.FindInputObjectArgTypeName(createMutation)
		// Return type from mutation
		g.ReturnType = createMutation.Type.GetTypeName()
	}

	// Build schema fields from create input type
	if g.CreateInputType != "" {
		fields, err := lang.BuildTerraformFields(s, g.CreateInputType)
		if err != nil {
			return fmt.Errorf("terraform generator: building fields for %q: %w", g.CreateInputType, err)
		}
		g.SchemaFields = fields
	}

	// Apply computed/sensitive/immutable overrides
	g.applyFieldOverrides()

	// Resolve Update mutation
	updateMutationName := tfCfg.UpdateMutation
	if updateMutationName == "" {
		updateMutationName = findMutationByVerb(pkgConfig.Mutations, "update")
	}
	if updateMutationName != "" && !tfCfg.NoUpdateMutation {
		g.HasUpdate = true
		g.UpdateMethod = lang.DeriveMethodName(updateMutationName)
		updateMutation, err := s.LookupMutationByName(updateMutationName)
		if err == nil {
			g.UpdateInputType = lang.FindInputObjectArgTypeName(updateMutation)
		}
	}

	// Resolve Delete mutation
	deleteMutationName := tfCfg.DeleteMutation
	if deleteMutationName == "" {
		deleteMutationName = findMutationByVerb(pkgConfig.Mutations, "delete")
	}
	if deleteMutationName != "" {
		g.DeleteMethod = lang.DeriveMethodName(deleteMutationName)
	}

	// Read method from config
	g.ReadMethod = tfCfg.ReadMethod

	return nil
}

// applyFieldOverrides applies Computed, Sensitive, ForceNew, and ConflictsWith overrides
// from TerraformConfig to the populated SchemaFields.
func (g *Generator) applyFieldOverrides() {
	if g.TerraformConfig == nil {
		return
	}
	cfg := g.TerraformConfig

	computedSet := stringSet(cfg.ComputedFields)
	sensitiveSet := stringSet(cfg.SensitiveFields)
	immutableSet := stringSet(cfg.ImmutableFields)
	skipReadSet := stringSet(cfg.SkipSetOnRead)

	for i := range g.SchemaFields {
		f := &g.SchemaFields[i]
		if computedSet[f.Name] {
			f.Computed = true
			f.Required = false
			f.Optional = true
		}
		if sensitiveSet[f.Name] {
			f.Sensitive = true
		}
		if immutableSet[f.Name] || cfg.NoUpdateMutation {
			f.ForceNew = true
		}
		if skipReadSet[f.Name] {
			f.SkipOnRead = true
		}
	}

	// ConflictingFields: [[a,b]] → a ConflictsWith b and b ConflictsWith a
	for _, pair := range cfg.ConflictingFields {
		if len(pair) < 2 {
			continue
		}
		for i := range g.SchemaFields {
			f := &g.SchemaFields[i]
			for _, name := range pair {
				if f.Name == name {
					for _, other := range pair {
						if other != name {
							f.ConflictsWith = append(f.ConflictsWith, other)
						}
					}
				}
			}
		}
	}
}

// Execute renders the terraform templates and writes output files.
func (g *Generator) Execute(genConfig *config.GeneratorConfig, pkgConfig *config.PackageConfig) error {
	if pkgConfig == nil || pkgConfig.Terraform == nil {
		return nil
	}

	destDir := pkgConfig.GetDestinationPath()
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		if err = os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("terraform generator: creating dest dir: %w", err)
		}
	}

	templateDir := "templates/terraform"
	if genConfig != nil && genConfig.TemplateDir != "" {
		var err error
		templateDir, err = codegen.RenderStringFromGenerator(genConfig.TemplateDir, g)
		if err != nil {
			return fmt.Errorf("terraform generator: rendering template dir: %w", err)
		}
	}

	base := "resource_newrelic_" + pkgConfig.Terraform.ResourceName
	var generatedFiles []string

	// resource.go — always regenerated
	resourceFile := fmt.Sprintf("%s/%s.go", destDir, base)
	if err := writeTemplate(g, templateDir, "resource.go.tmpl", resourceFile, destDir); err != nil {
		return fmt.Errorf("terraform generator: %w", err)
	}
	generatedFiles = append(generatedFiles, resourceFile)

	// structures.go — always regenerated
	structuresFile := fmt.Sprintf("%s/structures_newrelic_%s.go", destDir, pkgConfig.Terraform.ResourceName)
	if err := writeTemplate(g, templateDir, "structures.go.tmpl", structuresFile, destDir); err != nil {
		return fmt.Errorf("terraform generator: %w", err)
	}
	generatedFiles = append(generatedFiles, structuresFile)

	// provider_registration.txt — always regenerated
	regFile := fmt.Sprintf("%s/provider_registration.txt", destDir)
	if err := writeTemplate(g, templateDir, "provider_registration.txt.tmpl", regFile, destDir); err != nil {
		return fmt.Errorf("terraform generator: %w", err)
	}
	generatedFiles = append(generatedFiles, regFile)

	// resource_test.go — written once only
	testFile := fmt.Sprintf("%s/%s_test.go", destDir, base)
	if !fileExists(testFile) {
		if err := writeTemplate(g, templateDir, "resource_test.go.tmpl", testFile, destDir); err != nil {
			log.Warnf("terraform generator: error generating test scaffold: %v", err)
		} else {
			generatedFiles = append(generatedFiles, testFile)
		}
	}

	output.PrintSuccessMessage(destDir, generatedFiles)
	return nil
}

func writeTemplate(g *Generator, templateDir, templateName, destFile, destDir string) error {
	c := codegen.CodeGen{
		TemplateDir:     templateDir,
		TemplateName:    templateName,
		DestinationFile: destFile,
		DestinationDir:  destDir,
	}
	return c.WriteFile(g)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func stringSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

// findMutationByVerb returns the first mutation whose name contains the verb (case-insensitive).
func findMutationByVerb(mutations []config.MutationConfig, verb string) string {
	for _, m := range mutations {
		if strings.Contains(strings.ToLower(m.Name), strings.ToLower(verb)) {
			return m.Name
		}
	}
	return ""
}
