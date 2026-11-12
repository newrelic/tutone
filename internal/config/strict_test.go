package config

import (
	"os"
	"path/filepath"
	"testing"

	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, yml string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "tutone.yml")
	require.NoError(t, os.WriteFile(path, []byte(yml), 0o600))

	return path
}

func TestLoadConfig_UnknownKeyFailsAndNamesTheKey(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		yml string
		key string
	}{
		"top level": {
			yml: "endpoint: https://x\nnot_a_real_key: 1\n",
			key: "not_a_real_key",
		},
		"in a package": {
			yml: "packages:\n  - name: p\n    pacakge_path: typo\n",
			key: "pacakge_path",
		},
		"in a type": {
			yml: "packages:\n  - name: p\n    types:\n      - name: T\n        skip_type_creat: true\n",
			key: "skip_type_creat",
		},
		"in a field override": {
			yml: "packages:\n  - name: p\n    types:\n      - name: T\n        fields:\n          - name: f\n            pointr: true\n",
			key: "pointr",
		},
		"in a mutation": {
			yml: "packages:\n  - name: p\n    mutations:\n      - name: m\n        exclude_argument: [v]\n",
			key: "exclude_argument",
		},
		"in an endpoint": {
			yml: "packages:\n  - name: p\n    queries:\n      - path: [actor]\n        endpoints:\n          - name: e\n            include_implementation: [X]\n",
			key: "include_implementation",
		},
		"keep_existing misspelled on a type": {
			yml: "packages:\n  - name: p\n    types:\n      - name: T\n        keep_exisiting: true\n",
			key: "keep_exisiting",
		},
		"keep_existing misspelled on a mutation": {
			yml: "packages:\n  - name: p\n    mutations:\n      - name: m\n        keep_exisiting: true\n",
			key: "keep_exisiting",
		},
		"keep_existing misspelled on an endpoint": {
			yml: "packages:\n  - name: p\n    queries:\n      - path: [actor]\n        endpoints:\n          - name: e\n            keep_exisiting: true\n",
			key: "keep_exisiting",
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg, err := LoadConfig(writeConfig(t, tc.yml))
			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tc.key, "the error must name the offending key")
		})
	}
}

func TestLoadConfig_ErrorNamesTheFile(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, "bogus: 1\n")

	_, err := LoadConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
}

func TestLoadConfig_EveryKeyThisWorkAddsIsAccepted(t *testing.T) {
	t.Parallel()

	yml := `
packages:
  - name: agentApplications
    file_name: agentapplications
    go_package_name: agentapplications
    client_type_name: AgentApplications
    preserve_hand_written: false
    types:
      - name: EntityManagementEntityInterface
        include_implementations: [PipelineRule]
        fields:
          - name: enabled
            pointer: true
    mutations:
      - name: entityRelationshipUserDefinedCreateOrReplace
        exclude_arguments: [version]
        rename_arguments:
          type: relationtype
    queries:
      - path: [actor, entityManagement]
        endpoints:
          - name: entity
            method_name: GetEntityForPipeline
            include_implementations: [PipelineRule]
`

	cfg, err := LoadConfig(writeConfig(t, yml))
	require.NoError(t, err)

	pkg := cfg.Packages[0]
	assert.Equal(t, "agentapplications", pkg.GetFileName())
	assert.Equal(t, "agentapplications", pkg.GetGoPackageName())
	assert.Equal(t, "AgentApplications", pkg.GetClientTypeName())
	assert.False(t, pkg.GetPreserveHandWritten())
	assert.Equal(t, []string{"PipelineRule"}, pkg.Types[0].IncludeImplementations)
	assert.Equal(t, map[string]string{"type": "relationtype"}, pkg.Mutations[0].RenameArguments)
	assert.Equal(t, "GetEntityForPipeline", pkg.Queries[0].Endpoints[0].MethodName)
	assert.Equal(t, []string{"PipelineRule"}, pkg.Queries[0].Endpoints[0].IncludeImplementations)
}

func TestLoadConfig_KeepExistingIsAcceptedOnTypesMutationsAndEndpoints(t *testing.T) {
	t.Parallel()

	yml := `
packages:
  - name: notifications
    types:
      - name: Handwritten
        keep_existing: true
      - name: Generated
    mutations:
      - name: aiNotificationsCreateChannel
        keep_existing: true
      - name: aiNotificationsDeleteChannel
    queries:
      - path: [actor, account, aiNotifications]
        endpoints:
          - name: destinations
            keep_existing: true
      - path: [actor, organization, aiNotifications]
        endpoints:
          - name: destinations
            keep_existing: true
          - name: channels
`

	cfg, err := LoadConfig(writeConfig(t, yml))
	require.NoError(t, err)

	pkg := cfg.Packages[0]
	assert.True(t, pkg.Types[0].KeepExisting)
	assert.False(t, pkg.Types[1].KeepExisting, "off unless set")
	assert.True(t, pkg.Mutations[0].KeepExisting)
	assert.False(t, pkg.Mutations[1].KeepExisting)
	assert.True(t, pkg.Queries[0].Endpoints[0].KeepExisting)
	assert.True(t, pkg.Queries[1].Endpoints[0].KeepExisting)
	assert.False(t, pkg.Queries[1].Endpoints[1].KeepExisting)

	assert.Equal(t,
		[]string{"Handwritten", "aiNotificationsCreateChannel", "destinations", "destinations"},
		keepExistingNames(pkg))
}

func TestLoadConfig_IncludeOptionalArgumentsIsAcceptedOnEndpointsAndMutations(t *testing.T) {
	t.Parallel()

	yml := `
packages:
  - name: things
    mutations:
      - name: thingUpdate
        include_optional_arguments: true
        argument_go_types:
          version: "*int"
      - name: thingDelete
    queries:
      - path: [actor]
        endpoints:
          - name: things
            include_optional_arguments: true
            exclude_arguments: [filter]
          - name: others
`

	cfg, err := LoadConfig(writeConfig(t, yml))
	require.NoError(t, err)

	pkg := cfg.Packages[0]
	assert.True(t, pkg.Mutations[0].IncludeOptionalArguments)
	assert.False(t, pkg.Mutations[1].IncludeOptionalArguments, "off unless set")
	assert.True(t, pkg.Queries[0].Endpoints[0].IncludeOptionalArguments)
	assert.False(t, pkg.Queries[0].Endpoints[1].IncludeOptionalArguments)
}

func TestLoadConfig_KeepExistingWithoutPreservingWarns(t *testing.T) {
	// Not parallel: the hook is on the standard logger.
	hook := logtest.NewGlobal()
	t.Cleanup(hook.Reset)

	warnings := func() []*log.Entry {
		var out []*log.Entry

		for _, e := range hook.AllEntries() {
			if e.Level == log.WarnLevel {
				out = append(out, e)
			}
		}

		return out
	}

	preserved := "packages:\n  - name: kept\n    mutations:\n      - name: m\n        keep_existing: true\n"
	_, err := LoadConfig(writeConfig(t, preserved))
	require.NoError(t, err)
	assert.Empty(t, warnings(), "preserving is the default, so keep_existing works")

	notPreserved := "packages:\n  - name: replaced\n    preserve_hand_written: false\n    types:\n      - name: T\n        keep_existing: true\n"
	_, err = LoadConfig(writeConfig(t, notPreserved))
	require.NoError(t, err)

	got := warnings()
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Message, "keep_existing")
	assert.Equal(t, "replaced", got[0].Data["package"])
	assert.Equal(t, []string{"T"}, got[0].Data["entries"])
}

func TestLoadConfig_KeepRemovedFieldsDefaultsToOff(t *testing.T) {
	t.Parallel()

	cfg, err := LoadConfig(writeConfig(t, "packages:\n  - name: a\n  - name: b\n    keep_removed_fields: true\n"))
	require.NoError(t, err)

	assert.False(t, cfg.Packages[0].KeepRemovedFields)
	assert.True(t, cfg.Packages[1].KeepRemovedFields)
}

func TestLoadConfig_KeepRemovedFieldsWithoutPreservingWarns(t *testing.T) {
	// Not parallel: the hook is on the standard logger.
	hook := logtest.NewGlobal()
	t.Cleanup(hook.Reset)

	_, err := LoadConfig(writeConfig(t, "packages:\n  - name: kept\n    keep_removed_fields: true\n"))
	require.NoError(t, err)
	assert.Empty(t, hook.AllEntries(), "preserving is the default, so keep_removed_fields works")

	_, err = LoadConfig(writeConfig(t, "packages:\n  - name: replaced\n    preserve_hand_written: false\n    keep_removed_fields: true\n"))
	require.NoError(t, err)

	require.Len(t, hook.AllEntries(), 1)
	assert.Equal(t, log.WarnLevel, hook.LastEntry().Level)
	assert.Contains(t, hook.LastEntry().Message, "keep_removed_fields")
	assert.Equal(t, "replaced", hook.LastEntry().Data["package"])

	hook.Reset()

	_, err = LoadConfig(writeConfig(t, "packages:\n  - name: off\n    preserve_hand_written: false\n"))
	require.NoError(t, err)
	assert.Empty(t, hook.AllEntries(), "nothing to warn about when the option is off")
}

func TestLoadConfig_ExistingStyleConfigStillLoads(t *testing.T) {
	t.Parallel()

	// Keys the existing newrelic-client-go config relies on.
	yml := `
log_level: debug
endpoint: https://api.newrelic.com/graphql
auth:
  header: Api-Key
  api_key_env_var: TUTONE_API_KEY
cache:
  enable: true
  schema_file: schema.json
generators:
  - name: typegen
    fileName: types.go
  - name: nerdgraphclient
    fileName: "{{.PackageName}}_api.go"
packages:
  - name: alerts
    path: pkg/alerts
    import_path: github.com/newrelic/newrelic-client-go/pkg/alerts
    imports:
      - github.com/newrelic/newrelic-client-go/pkg/errors
    generators: [typegen, nerdgraphclient]
    types:
      - name: ID
        create_as: string
      - name: X
        field_type_override: "*string"
        skip_type_create: true
        generate_struct_getters: true
        interface_methods: [ImplementsFoo()]
        skip_fields: [a]
        struct_tags:
          tags: [json]
          omitempty: false
    mutations:
      - name: alertsPolicyCreate
        max_query_field_depth: 2
        argument_type_overrides:
          accountId: Int!
        exclude_fields: [notes]
    queries:
      - path: [actor, account]
        endpoints:
          - name: policy
            max_query_field_depth: 3
            include_arguments: [id]
            exclude_fields: [x]
`

	cfg, err := LoadConfig(writeConfig(t, yml))
	require.NoError(t, err)
	assert.Equal(t, "alerts", cfg.Packages[0].Name)
	assert.True(t, cfg.Packages[0].GetPreserveHandWritten(), "preserving is the default")
}

func TestPackageConfig_NamingDefaults(t *testing.T) {
	t.Parallel()

	p := &PackageConfig{Name: "agentApplications"}
	assert.Equal(t, "agentApplications", p.GetFileName())
	assert.Equal(t, "agentapplications", p.GetGoPackageName())
	assert.Equal(t, "AgentApplications", p.GetClientTypeName())
	assert.Equal(t, "AgentApplications", p.DefaultClientTypeName())
	assert.True(t, p.GetPreserveHandWritten())

	p.ClientTypeName = "OrganizationManagement"
	assert.Equal(t, "OrganizationManagement", p.GetClientTypeName())
	assert.Equal(t, "AgentApplications", p.DefaultClientTypeName(), "what a template that predates client_type_name writes")
}

func TestLoadConfig_DeprecatedEndpointArgumentTypeOverridesStillLoads(t *testing.T) {
	t.Parallel()

	// Documented, accepted by the old loader, and ignored. Existing configs have it.
	yml := `
packages:
  - name: cloud
    queries:
      - path: [actor, cloud]
        endpoints:
          - name: linkedAccounts
            include_arguments: [provider]
            argument_type_overrides:
              - accountId: "Int!"
`

	cfg, err := LoadConfig(writeConfig(t, yml))
	require.NoError(t, err)
	assert.Equal(t, []map[string]string{{"accountId": "Int!"}}, cfg.Packages[0].Queries[0].Endpoints[0].ArgumentTypeOverrides)
}

func TestLoadConfig_TypoNextToADeprecatedKeyStillFails(t *testing.T) {
	t.Parallel()

	yml := `
packages:
  - name: cloud
    queries:
      - path: [actor, cloud]
        endpoints:
          - name: linkedAccounts
            argument_type_overrides:
              - accountId: "Int!"
            include_argumnts: [provider]
`

	_, err := LoadConfig(writeConfig(t, yml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "include_argumnts")
	assert.NotContains(t, err.Error(), "argument_type_overrides")
}

func TestDuplicateTypeNames(t *testing.T) {
	t.Parallel()

	assert.Empty(t, duplicateTypeNames(nil))
	assert.Empty(t, duplicateTypeNames([]TypeConfig{{Name: "A"}, {Name: "B"}}))
	assert.Equal(t, []string{"A"}, duplicateTypeNames([]TypeConfig{{Name: "A"}, {Name: "B"}, {Name: "A"}, {Name: "A"}}), "reported once however often it repeats")
}
