package generate

import (
	"fmt"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
)

// applyImplementationFilters narrows the PossibleTypes of every interface named
// by an include_implementations setting in pkg, and returns a function that puts
// the schema back as it was.
//
// The schema is shared by every package in one run. Without the restore, the
// filter for one package would still be in force for the next, and a single
// generate over all packages would give each NGEP package the entity types of
// the one before it.
func applyImplementationFilters(s *schema.Schema, pkg *config.PackageConfig) (restore func(), err error) {
	includes, err := implementationFilters(s, pkg)
	if err != nil {
		return func() {}, err
	}

	originals := make(map[string][]schema.TypeRef, len(includes))

	restore = func() {
		for name, possible := range originals {
			if t, lookupErr := s.LookupTypeByName(name); lookupErr == nil {
				t.PossibleTypes = possible
			}
		}
	}

	names := make([]string, 0, len(includes))
	for name := range includes {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		t, lookupErr := s.LookupTypeByName(name)
		if lookupErr != nil {
			restore()
			return func() {}, lookupErr
		}

		keep := includes[name]

		var filtered []schema.TypeRef

		for _, impl := range t.PossibleTypes {
			if keep[impl.Name] {
				filtered = append(filtered, impl)
			}
		}

		for impl := range keep {
			if !hasPossibleType(t.PossibleTypes, impl) {
				restore()

				return func() {}, fmt.Errorf("package %q: include_implementations names %q, which does not implement %q", pkg.Name, impl, name)
			}
		}

		originals[name] = t.PossibleTypes
		t.PossibleTypes = filtered

		log.WithFields(log.Fields{
			"interface": name,
			"kept":      len(filtered),
		}).Info("narrowed interface implementations for package")
	}

	return restore, nil
}

// implementationFilters collects, per interface type name, the implementations
// to keep. Settings on a type and on an endpoint that name the same interface
// are combined.
func implementationFilters(s *schema.Schema, pkg *config.PackageConfig) (map[string]map[string]bool, error) {
	includes := map[string]map[string]bool{}

	add := func(interfaceName string, impls []string) {
		if len(impls) == 0 {
			return
		}

		if includes[interfaceName] == nil {
			includes[interfaceName] = map[string]bool{}
		}

		for _, impl := range impls {
			includes[interfaceName][impl] = true
		}
	}

	for _, tc := range pkg.Types {
		if len(tc.IncludeImplementations) == 0 {
			continue
		}

		if _, err := s.LookupTypeByName(tc.Name); err != nil {
			return nil, fmt.Errorf("package %q: include_implementations is set on type %q, which is not in the schema: %w", pkg.Name, tc.Name, err)
		}

		add(tc.Name, tc.IncludeImplementations)
	}

	for _, q := range pkg.Queries {
		for _, e := range q.Endpoints {
			if len(e.IncludeImplementations) == 0 {
				continue
			}

			returned, err := endpointReturnType(s, q.Path, e.Name)
			if err != nil {
				return nil, fmt.Errorf("package %q: include_implementations is set on endpoint %q: %w", pkg.Name, e.Name, err)
			}

			add(returned, e.IncludeImplementations)
		}
	}

	return includes, nil
}

// endpointReturnType finds the name of the type an endpoint returns.
func endpointReturnType(s *schema.Schema, path []string, endpoint string) (string, error) {
	types, err := s.LookupQueryTypesByFieldPath(path)
	if err != nil {
		return "", err
	}

	last := types[len(types)-1]

	for _, f := range last.Fields {
		if f.Name == endpoint {
			return f.Type.GetTypeName(), nil
		}
	}

	return "", fmt.Errorf("no field %q on type %q", endpoint, last.Name)
}

func hasPossibleType(possible []schema.TypeRef, name string) bool {
	for _, p := range possible {
		if p.Name == name {
			return true
		}
	}

	return false
}

// warnUnknownTypes logs, and returns, every type in the package config whose
// settings are not all applied. One the schema does not have is ignored entirely; a
// typo or a renamed type is the usual cause. One whose name differs from the
// schema's only in case, usually the Go spelling such as NRQL for Nrql, still
// expands the type and applies create_as, skip_type_create, generate_struct_getters
// and interface_methods, so it is reported only when it has a setting that needs the
// exact name. It warns rather than fails so existing configs that name a
// since-removed type keep working.
func warnUnknownTypes(s *schema.Schema, pkg *config.PackageConfig) []string {
	var unknown []string

	for _, tc := range pkg.Types {
		if _, err := s.LookupTypeByName(tc.Name); err == nil {
			continue
		}

		fields := log.Fields{
			"package": pkg.Name,
			"type":    tc.Name,
		}

		match := lookupTypeByNameIgnoringCase(s, tc.Name)
		if match == nil {
			unknown = append(unknown, tc.Name)
			log.WithFields(fields).Warn("type in the package config is not in the schema, so its settings are ignored; use the GraphQL name")

			continue
		}

		if ignored := exactNameSettings(tc); len(ignored) > 0 {
			unknown = append(unknown, tc.Name)
			fields["schema_type"] = match.Name
			fields["ignored"] = ignored
			log.WithFields(fields).Warn("type in the package config differs from the schema's name only in case, so the settings that need the exact name are ignored; renaming it applies them and changes the generated code")
		}
	}

	return unknown
}

func lookupTypeByNameIgnoringCase(s *schema.Schema, name string) *schema.Type {
	for _, t := range s.Types {
		if t != nil && strings.EqualFold(t.Name, name) {
			return t
		}
	}

	return nil
}

// exactNameSettings returns the settings in tc that apply only to the type with exactly its name.
func exactNameSettings(tc config.TypeConfig) []string {
	var names []string

	if tc.FieldTypeOverride != "" {
		names = append(names, "field_type_override")
	}

	if len(tc.SkipFields) > 0 {
		names = append(names, "skip_fields")
	}

	if tc.StructTags != nil {
		names = append(names, "struct_tags")
	}

	if len(tc.Fields) > 0 {
		names = append(names, "fields")
	}

	return names
}
