//go:build unit
// +build unit

package schema

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/tutone/internal/config"
)

func TestMutationInMutations(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		Name           string
		Mutations      []config.MutationConfig
		ExpectedResult bool
	}{
		"simple string": {
			Name: "asdf",
			Mutations: []config.MutationConfig{
				{
					Name: "asdf",
				},
			},
			ExpectedResult: true,
		},
		"no match": {
			Name: "bar",
			Mutations: []config.MutationConfig{
				{
					Name: "foo",
				},
			},
			ExpectedResult: false,
		},
		"regexp match simple": {
			Name: "edgeCreateTraceFilterRules",
			Mutations: []config.MutationConfig{
				{
					Name: "edge.*",
				},
			},
			ExpectedResult: true,
		},
		"regexp match complex": {
			Name: "edgeCreateTraceFilterRules",
			Mutations: []config.MutationConfig{
				{
					Name: "edge(Create|Delete)TraceFilterRules",
				},
			},
			ExpectedResult: true,
		},
		"regexp no match": {
			Name: "bar",
			Mutations: []config.MutationConfig{
				{
					Name: "/foo(Create|Delete)/",
				},
			},
			ExpectedResult: false,
		},
	}

	for caseName, tc := range cases {
		result := mutationNameInMutations(tc.Name, tc.Mutations)
		assert.Equal(t, tc.ExpectedResult, result, caseName)
	}
}

func TestExpandTypes(t *testing.T) {
	t.Parallel()

	// schema cached by 'make test-prep'
	s, err := Load("../../testdata/schema.json")
	assert.NoError(t, err)

	cases := map[string]struct {
		Types         []config.TypeConfig
		Mutations     []config.MutationConfig
		ExpectErr     bool
		ExpectReason  string
		ExpectedNames []string
	}{
		"single type": {
			Types: []config.TypeConfig{{
				Name: "AlertsPolicy",
			}},
			Mutations: []config.MutationConfig{},
			ExpectedNames: []string{"AlertsIncidentPreference", "AlertsPolicy", "EntityGuid", "ID", "Int", "String"},
		},
		"single mutation": {
			Types: []config.TypeConfig{},
			Mutations: []config.MutationConfig{{
				Name: "alertsNrqlConditionBaselineCreate",
			}},
			ExpectedNames: []string{"AccountOutline", "AccountReference", "AlertableEntityOutline", "AlertsFillOption", "AlertsNrqlBaselineCondition", "AlertsNrqlBaselineDirection", "AlertsNrqlCondition", "AlertsNrqlConditionBaselineInput", "AlertsNrqlConditionExpiration", "AlertsNrqlConditionExpirationInput", "AlertsNrqlConditionPrediction", "AlertsNrqlConditionPriority", "AlertsNrqlConditionQuery", "AlertsNrqlConditionQueryInput", "AlertsNrqlConditionSignal", "AlertsNrqlConditionSignalInput", "AlertsNrqlConditionTerms", "AlertsNrqlConditionTermsOperator", "AlertsNrqlConditionTermsWithPrediction", "AlertsNrqlConditionThresholdOccurrences", "AlertsNrqlConditionType", "AlertsNrqlDynamicConditionTermsInput", "AlertsNrqlDynamicConditionTermsOperator", "AlertsNrqlOutlierCondition", "AlertsNrqlSignalSeasonality", "AlertsNrqlStaticCondition", "AlertsNrqlStaticConditionValueFunction", "AlertsNrqlTerms", "AlertsOutlierConfiguration", "AlertsOutlierDbScanConfiguration", "AlertsSignalAggregationMethod", "AlertsViolationTimeLimit", "ApmApplicationEntityOutline", "ApmApplicationRunningAgentVersions", "ApmApplicationSettings", "ApmApplicationSummaryData", "ApmBrowserApplicationEntityOutline", "ApmBrowserApplicationSummaryData", "ApmDatabaseInstanceEntityOutline", "ApmExternalServiceEntityOutline", "ApmExternalServiceSummaryData", "Boolean", "BrowserAgentInstallType", "BrowserApplicationEntityOutline", "BrowserApplicationRunningAgentVersions", "BrowserApplicationSettings", "BrowserApplicationSummaryData", "DashboardEntityOutline", "DashboardEntityOwnerInfo", "DashboardEntityOwnerType", "DashboardEntityPermissions", "DateTime", "EntityAlertSeverity", "EntityGoldenContext", "EntityGoldenContextInput", "EntityGoldenContextScopedGoldenMetrics", "EntityGoldenContextScopedGoldenTags", "EntityGoldenEventObjectId", "EntityGoldenMetric", "EntityGoldenMetricDefinition", "EntityGoldenMetricUnit", "EntityGoldenNrqlTimeWindowInput", "EntityGoldenOriginalDefinitionWithSelector", "EntityGoldenOriginalQueryWithSelector", "EntityGoldenTag", "EntityGuid", "EntityOutline", "EntityTag", "EntityType", "EpochMilliseconds", "ExternalEntityOutline", "Float", "GenericEntityOutline", "GenericInfrastructureEntityOutline", "ID", "InfrastructureAwsLambdaFunctionEntityOutline", "InfrastructureHostEntityOutline", "InfrastructureHostSummaryData", "InfrastructureIntegrationEntityOutline", "Int", "KeyTransactionEntityOutline", "Minutes", "MobileAppSummaryData", "MobileApplicationEntityOutline", "Nrql", "Seconds", "SecureCredentialEntityOutline", "SecureCredentialSummaryData", "SemVer", "ServiceLevelActor", "ServiceLevelActorType", "ServiceLevelDefinition", "ServiceLevelEvents", "ServiceLevelEventsQuery", "ServiceLevelEventsQuerySelect", "ServiceLevelEventsQuerySelectFunction", "ServiceLevelIndicator", "ServiceLevelIndicatorResultQueries", "ServiceLevelMetadata", "ServiceLevelObjective", "ServiceLevelObjectiveResultQueries", "ServiceLevelObjectiveRollingTimeWindow", "ServiceLevelObjectiveRollingTimeWindowUnit", "ServiceLevelObjectiveTimeWindow", "ServiceLevelResultQuery", "ServiceLevelSystemIdentityActor", "ServiceLevelUserActor", "String", "SyntheticMonitorEntityOutline", "SyntheticMonitorStatus", "SyntheticMonitorSummaryData", "SyntheticMonitorType", "TeamEntityOutline", "ThirdPartyServiceEntityOutline", "TimeWindowInput", "UnavailableEntityOutline", "UserReference", "WorkloadEntityOutline", "WorkloadStatus", "WorkloadStatusSource", "WorkloadStatusValue"},
		},
		"sample interface type": {
			Types: []config.TypeConfig{{
				Name: "CloudProvider",
			}},
			Mutations:     []config.MutationConfig{},
			ExpectedNames: []string{"Boolean", "CloudAwsEuSovereignProvider", "CloudAwsGovCloudProvider", "CloudAwsProvider", "CloudBaseProvider", "CloudDashboardTemplate", "CloudGcpProvider", "CloudProvider", "CloudService", "CloudTemplateParam", "EpochSeconds", "Int", "String"},
		},
		"nested slice of interface": {
			Types: []config.TypeConfig{{
				Name: "CloudLinkedAccount",
			}},
			Mutations:     []config.MutationConfig{},
			ExpectedNames: []string{"Boolean", "CloudAlbIntegration", "CloudApigatewayIntegration", "CloudAutoscalingIntegration", "CloudAwsAppsyncIntegration", "CloudAwsAthenaIntegration", "CloudAwsAutoDiscoveryIntegration", "CloudAwsCognitoIntegration", "CloudAwsConnectIntegration", "CloudAwsDirectconnectIntegration", "CloudAwsDocdbIntegration", "CloudAwsEuSovereignProvider", "CloudAwsFsxIntegration", "CloudAwsGlueIntegration", "CloudAwsGovCloudProvider", "CloudAwsKinesisanalyticsIntegration", "CloudAwsMediaconvertIntegration", "CloudAwsMediapackagevodIntegration", "CloudAwsMetadataEuSovereignIntegration", "CloudAwsMetadataGovIntegration", "CloudAwsMetadataIntegration", "CloudAwsMqIntegration", "CloudAwsMsElasticacheEuSovereignIntegration", "CloudAwsMsElasticacheGovIntegration", "CloudAwsMsElasticacheIntegration", "CloudAwsMskIntegration", "CloudAwsNeptuneIntegration", "CloudAwsProvider", "CloudAwsQldbIntegration", "CloudAwsRoute53resolverIntegration", "CloudAwsStatesIntegration", "CloudAwsTagsGlobalEuSovereignIntegration", "CloudAwsTagsGlobalGovIntegration", "CloudAwsTagsGlobalIntegration", "CloudAwsTransitgatewayIntegration", "CloudAwsWafIntegration", "CloudAwsWafv2Integration", "CloudAwsXrayIntegration", "CloudAzureApimanagementIntegration", "CloudAzureAppgatewayIntegration", "CloudAzureAppserviceIntegration", "CloudAzureAutoDiscoveryIntegration", "CloudAzureContainersIntegration", "CloudAzureCosmosdbIntegration", "CloudAzureCostmanagementIntegration", "CloudAzureDatafactoryIntegration", "CloudAzureEventhubIntegration", "CloudAzureExpressrouteIntegration", "CloudAzureFirewallsIntegration", "CloudAzureFrontdoorIntegration", "CloudAzureFunctionsIntegration", "CloudAzureKeyvaultIntegration", "CloudAzureLoadbalancerIntegration", "CloudAzureLogicappsIntegration", "CloudAzureMachinelearningIntegration", "CloudAzureMariadbIntegration", "CloudAzureMonitorIntegration", "CloudAzureMysqlIntegration", "CloudAzureMysqlflexibleIntegration", "CloudAzurePostgresqlIntegration", "CloudAzurePostgresqlflexibleIntegration", "CloudAzurePowerbidedicatedIntegration", "CloudAzureRediscacheIntegration", "CloudAzureServicebusIntegration", "CloudAzureSqlIntegration", "CloudAzureSqlmanagedIntegration", "CloudAzureStorageIntegration", "CloudAzureVirtualmachineIntegration", "CloudAzureVirtualnetworksIntegration", "CloudAzureVmsIntegration", "CloudAzureVpngatewaysIntegration", "CloudBaseIntegration", "CloudBaseProvider", "CloudBillingIntegration", "CloudCciAwsS3Integration", "CloudCloudfrontIntegration", "CloudCloudtrailIntegration", "CloudConfluentKafkaConnectorResourceIntegration", "CloudConfluentKafkaFlinkResourceIntegration", "CloudConfluentKafkaKsqlResourceIntegration", "CloudConfluentKafkaResourceIntegration", "CloudDashboardTemplate", "CloudDynamodbIntegration", "CloudEbsIntegration", "CloudEc2Integration", "CloudEcsIntegration", "CloudEfsIntegration", "CloudElasticacheIntegration", "CloudElasticbeanstalkIntegration", "CloudElasticsearchIntegration", "CloudElbIntegration", "CloudEmrIntegration", "CloudFossaIssuesIntegration", "CloudGcpAiplatformIntegration", "CloudGcpAlloydbIntegration", "CloudGcpAppengineIntegration", "CloudGcpBigqueryIntegration", "CloudGcpBigtableIntegration", "CloudGcpComposerIntegration", "CloudGcpDataflowIntegration", "CloudGcpDataprocIntegration", "CloudGcpDatastoreIntegration", "CloudGcpFirebasedatabaseIntegration", "CloudGcpFirebasehostingIntegration", "CloudGcpFirebasestorageIntegration", "CloudGcpFirestoreIntegration", "CloudGcpFunctionsIntegration", "CloudGcpGenericIntegration", "CloudGcpInterconnectIntegration", "CloudGcpKubernetesIntegration", "CloudGcpLoadbalancingIntegration", "CloudGcpMemcacheIntegration", "CloudGcpProvider", "CloudGcpPubsubIntegration", "CloudGcpRedisIntegration", "CloudGcpRouterIntegration", "CloudGcpRunIntegration", "CloudGcpSpannerIntegration", "CloudGcpSqlIntegration", "CloudGcpStorageIntegration", "CloudGcpVmsIntegration", "CloudGcpVpcaccessIntegration", "CloudHealthIntegration", "CloudIamIntegration", "CloudIntegration", "CloudIotIntegration", "CloudKinesisFirehoseIntegration", "CloudKinesisIntegration", "CloudLambdaIntegration", "CloudLinkedAccount", "CloudMetricCollectionMode", "CloudOciLogsIntegration", "CloudOciMetadataAndTagsIntegration", "CloudProvider", "CloudRdsIntegration", "CloudRedshiftIntegration", "CloudRoute53Integration", "CloudS3Integration", "CloudSecurityHubIntegration", "CloudService", "CloudSesIntegration", "CloudSnsIntegration", "CloudSqsIntegration", "CloudTemplateParam", "CloudTrustedadvisorIntegration", "CloudVpcIntegration", "EpochSeconds", "Int", "String"},
		},
		"leveraging string replacer": {
			Types: []config.TypeConfig{},
			Mutations: []config.MutationConfig{{
				Name: "apiAccessCreateKeys",
			}},
			ExpectedNames: []string{"AccountReference", "ApiAccessCreateIngestKeyInput", "ApiAccessCreateInput", "ApiAccessCreateKeyResponse", "ApiAccessCreateUserKeyInput", "ApiAccessIngestKey", "ApiAccessIngestKeyError", "ApiAccessIngestKeyErrorType", "ApiAccessIngestKeyType", "ApiAccessKey", "ApiAccessKeyError", "ApiAccessKeyType", "ApiAccessUserKey", "ApiAccessUserKeyError", "ApiAccessUserKeyErrorType", "Boolean", "EpochSeconds", "ID", "Int", "String", "UserReference"},
		},
		"complicated cloud confirms complications": {
			Types: []config.TypeConfig{},
			Mutations: []config.MutationConfig{{
				Name: "cloudDisableIntegration",
			}},
			ExpectedNames: []string{"Boolean", "CloudAlbIntegration", "CloudApigatewayIntegration", "CloudAutoscalingIntegration", "CloudAwsAppsyncIntegration", "CloudAwsAthenaIntegration", "CloudAwsAutoDiscoveryIntegration", "CloudAwsCognitoIntegration", "CloudAwsConnectIntegration", "CloudAwsDirectconnectIntegration", "CloudAwsDisableIntegrationsInput", "CloudAwsDocdbIntegration", "CloudAwsEuSovereignDisableIntegrationsInput", "CloudAwsEuSovereignProvider", "CloudAwsFsxIntegration", "CloudAwsGlueIntegration", "CloudAwsGovCloudProvider", "CloudAwsGovcloudDisableIntegrationsInput", "CloudAwsKinesisanalyticsIntegration", "CloudAwsMediaconvertIntegration", "CloudAwsMediapackagevodIntegration", "CloudAwsMetadataEuSovereignIntegration", "CloudAwsMetadataGovIntegration", "CloudAwsMetadataIntegration", "CloudAwsMqIntegration", "CloudAwsMsElasticacheEuSovereignIntegration", "CloudAwsMsElasticacheGovIntegration", "CloudAwsMsElasticacheIntegration", "CloudAwsMskIntegration", "CloudAwsNeptuneIntegration", "CloudAwsProvider", "CloudAwsQldbIntegration", "CloudAwsRoute53resolverIntegration", "CloudAwsStatesIntegration", "CloudAwsTagsGlobalEuSovereignIntegration", "CloudAwsTagsGlobalGovIntegration", "CloudAwsTagsGlobalIntegration", "CloudAwsTransitgatewayIntegration", "CloudAwsWafIntegration", "CloudAwsWafv2Integration", "CloudAwsXrayIntegration", "CloudAzureApimanagementIntegration", "CloudAzureAppgatewayIntegration", "CloudAzureAppserviceIntegration", "CloudAzureAutoDiscoveryIntegration", "CloudAzureContainersIntegration", "CloudAzureCosmosdbIntegration", "CloudAzureCostmanagementIntegration", "CloudAzureDatafactoryIntegration", "CloudAzureDisableIntegrationsInput", "CloudAzureEventhubIntegration", "CloudAzureExpressrouteIntegration", "CloudAzureFirewallsIntegration", "CloudAzureFrontdoorIntegration", "CloudAzureFunctionsIntegration", "CloudAzureKeyvaultIntegration", "CloudAzureLoadbalancerIntegration", "CloudAzureLogicappsIntegration", "CloudAzureMachinelearningIntegration", "CloudAzureMariadbIntegration", "CloudAzureMonitorIntegration", "CloudAzureMysqlIntegration", "CloudAzureMysqlflexibleIntegration", "CloudAzurePostgresqlIntegration", "CloudAzurePostgresqlflexibleIntegration", "CloudAzurePowerbidedicatedIntegration", "CloudAzureRediscacheIntegration", "CloudAzureServicebusIntegration", "CloudAzureSqlIntegration", "CloudAzureSqlmanagedIntegration", "CloudAzureStorageIntegration", "CloudAzureVirtualmachineIntegration", "CloudAzureVirtualnetworksIntegration", "CloudAzureVmsIntegration", "CloudAzureVpngatewaysIntegration", "CloudBaseIntegration", "CloudBaseProvider", "CloudBillingIntegration", "CloudCciAwsDisableIntegrationsInput", "CloudCciAwsS3Integration", "CloudCloudfrontIntegration", "CloudCloudtrailIntegration", "CloudConfluentDisableIntegrationsInput", "CloudConfluentKafkaConnectorResourceIntegration", "CloudConfluentKafkaFlinkResourceIntegration", "CloudConfluentKafkaKsqlResourceIntegration", "CloudConfluentKafkaResourceIntegration", "CloudDashboardTemplate", "CloudDisableAccountIntegrationInput", "CloudDisableIntegrationPayload", "CloudDisableIntegrationsInput", "CloudDynamodbIntegration", "CloudEbsIntegration", "CloudEc2Integration", "CloudEcsIntegration", "CloudEfsIntegration", "CloudElasticacheIntegration", "CloudElasticbeanstalkIntegration", "CloudElasticsearchIntegration", "CloudElbIntegration", "CloudEmrIntegration", "CloudFossaDisableIntegrationsInput", "CloudFossaIssuesIntegration", "CloudGcpAiplatformIntegration", "CloudGcpAlloydbIntegration", "CloudGcpAppengineIntegration", "CloudGcpBigqueryIntegration", "CloudGcpBigtableIntegration", "CloudGcpComposerIntegration", "CloudGcpDataflowIntegration", "CloudGcpDataprocIntegration", "CloudGcpDatastoreIntegration", "CloudGcpDisableIntegrationsInput", "CloudGcpFirebasedatabaseIntegration", "CloudGcpFirebasehostingIntegration", "CloudGcpFirebasestorageIntegration", "CloudGcpFirestoreIntegration", "CloudGcpFunctionsIntegration", "CloudGcpGenericIntegration", "CloudGcpInterconnectIntegration", "CloudGcpKubernetesIntegration", "CloudGcpLoadbalancingIntegration", "CloudGcpMemcacheIntegration", "CloudGcpProvider", "CloudGcpPubsubIntegration", "CloudGcpRedisIntegration", "CloudGcpRouterIntegration", "CloudGcpRunIntegration", "CloudGcpSpannerIntegration", "CloudGcpSqlIntegration", "CloudGcpStorageIntegration", "CloudGcpVmsIntegration", "CloudGcpVpcaccessIntegration", "CloudHealthIntegration", "CloudIamIntegration", "CloudIntegration", "CloudIntegrationMutationError", "CloudIotIntegration", "CloudKinesisFirehoseIntegration", "CloudKinesisIntegration", "CloudLambdaIntegration", "CloudLinkedAccount", "CloudMetricCollectionMode", "CloudOciDisableIntegrationsInput", "CloudOciLogsIntegration", "CloudOciMetadataAndTagsIntegration", "CloudProvider", "CloudRdsIntegration", "CloudRedshiftIntegration", "CloudRoute53Integration", "CloudS3Integration", "CloudSecurityHubIntegration", "CloudService", "CloudSesIntegration", "CloudSnsIntegration", "CloudSqsIntegration", "CloudTemplateParam", "CloudTrustedadvisorIntegration", "CloudVpcIntegration", "EpochSeconds", "Int", "String"},
		},
	}

	for caseName, tc := range cases {
		t.Logf("case name: %s", caseName)

		testConfig := &config.PackageConfig{
			Types:     tc.Types,
			Mutations: tc.Mutations,
		}

		results, err := ExpandTypes(s, testConfig)
		if tc.ExpectErr {
			require.NotNil(t, err)
			require.Equal(t, err.Error(), tc.ExpectReason)
		} else {
			require.Nil(t, err)
		}

		names := []string{}
		for _, r := range *results {
			names = append(names, r.Name)
		}

		sort.Strings(names)
		sort.Strings(tc.ExpectedNames)

		assert.Equal(t, tc.ExpectedNames, names)
	}
}
