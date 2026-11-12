package generate

import (
	"github.com/newrelic/tutone/internal/codegen"
	"github.com/newrelic/tutone/internal/config"
	"github.com/newrelic/tutone/internal/schema"
)

// skipUnionsWithGoType sets skip_type_create on every union whose Go type the
// package already has, so no interface and no member types are generated for it and
// the fields of that type keep the type the package gave them.
//
// The package has a Go type for a union when the config names one with
// field_type_override, or when a hand-written file declares a type called after the
// union. When that file declares only <Name>Interface, that is the type of the fields,
// so field_type_override is set to it. A package may supply the Go type of a union by
// hand; generating another one would declare it twice.
func skipUnionsWithGoType(s *schema.Schema, pkg *config.PackageConfig) error {
	handWritten, err := codegen.ScanHandWritten(pkg.GetDestinationPath())
	if err != nil {
		return err
	}

	types := append([]config.TypeConfig(nil), pkg.Types...)

	for _, t := range s.Types {
		if t == nil || t.Kind != schema.KindUnion {
			continue
		}

		name := t.GetName()
		i := typeConfigIndex(types, t.Name)

		configured := i >= 0 && types[i].FieldTypeOverride != ""
		declared := handWritten.Types[name]

		if !configured && !declared && !handWritten.Types[name+"Interface"] {
			continue
		}

		if i < 0 {
			types = append(types, config.TypeConfig{Name: t.Name})
			i = len(types) - 1
		}

		types[i].SkipTypeCreate = true

		if !configured && !declared {
			types[i].FieldTypeOverride = name + "Interface"
		}
	}

	pkg.Types = types

	return nil
}

func typeConfigIndex(types []config.TypeConfig, name string) int {
	for i := range types {
		if types[i].Name == name {
			return i
		}
	}

	return -1
}
