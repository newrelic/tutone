// +build unit

package schema

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tj/assert"

	"github.com/newrelic/tutone/internal/config"
)

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
			ExpectedNames: []string{
				"AlertsPolicy",
				"ID",
				"Int",
				"AlertsIncidentPreference",
				"String",
			},
		},
		"single mutation": {
			Types: []config.TypeConfig{},
			Mutations: []config.MutationConfig{{
				Name: "alertsNrqlConditionBaselineCreate",
			}},
			ExpectedNames: []string{"AlertsFillOption", "AlertsNrqlBaselineCondition", "AlertsNrqlBaselineDirection", "AlertsNrqlCondition", "AlertsNrqlConditionBaselineInput", "AlertsNrqlConditionExpiration", "AlertsNrqlConditionExpirationInput", "AlertsNrqlConditionPriority", "AlertsNrqlConditionQuery", "AlertsNrqlConditionQueryInput", "AlertsNrqlConditionSignal", "AlertsNrqlConditionSignalInput", "AlertsNrqlConditionTerms", "AlertsNrqlConditionTermsOperator", "AlertsNrqlConditionThresholdOccurrences", "AlertsNrqlConditionType", "AlertsNrqlDynamicConditionTermsInput", "AlertsNrqlDynamicConditionTermsOperator", "AlertsNrqlOutlierCondition", "AlertsNrqlStaticCondition", "AlertsNrqlStaticConditionValueFunction", "AlertsViolationTimeLimit", "Boolean", "Float", "ID", "Int", "Seconds", "String"},
		},
		"sample interface type": {
			Types: []config.TypeConfig{{
				Name: "CloudProvider",
			}},
			Mutations:     []config.MutationConfig{},
			ExpectedNames: []string{"Boolean", "CloudAwsGovCloudProvider", "CloudAwsProvider", "CloudBaseProvider", "CloudGcpProvider", "CloudProvider", "CloudService", "EpochSeconds", "Int", "String"},
		},
		"nested slice of interface": {
			Types: []config.TypeConfig{{
				Name: "CloudLinkedAccount",
			}},
			Mutations:     []config.MutationConfig{},
			ExpectedNames: []string{"Boolean", "CloudAlbIntegration", "CloudApigatewayIntegration", "CloudAutoscalingIntegration", "CloudAwsAppsyncIntegration", "CloudAwsAthenaIntegration", "CloudAwsCognitoIntegration", "CloudAwsConnectIntegration", "CloudAwsDirectconnectIntegration", "CloudAwsDocdbIntegration", "CloudAwsFsxIntegration", "CloudAwsGlueIntegration", "CloudAwsGovCloudProvider", "CloudAwsKinesisanalyticsIntegration", "CloudAwsMediaconvertIntegration", "CloudAwsMediapackagevodIntegration", "CloudAwsMqIntegration", "CloudAwsMskIntegration", "CloudAwsNeptuneIntegration", "CloudAwsProvider", "CloudAwsQldbIntegration", "CloudAwsRoute53resolverIntegration", "CloudAwsStatesIntegration", "CloudAwsTransitgatewayIntegration", "CloudAwsWafIntegration", "CloudAwsWafv2Integration", "CloudAwsXrayIntegration", "CloudAzureApimanagementIntegration", "CloudAzureAppgatewayIntegration", "CloudAzureAppserviceIntegration", "CloudAzureContainersIntegration", "CloudAzureCosmosdbIntegration", "CloudAzureCostmanagementIntegration", "CloudAzureDatafactoryIntegration", "CloudAzureEventhubIntegration", "CloudAzureExpressrouteIntegration", "CloudAzureFirewallsIntegration", "CloudAzureFrontdoorIntegration", "CloudAzureFunctionsIntegration", "CloudAzureKeyvaultIntegration", "CloudAzureLoadbalancerIntegration", "CloudAzureLogicappsIntegration", "CloudAzureMachinelearningIntegration", "CloudAzureMariadbIntegration", "CloudAzureMysqlIntegration", "CloudAzurePostgresqlIntegration", "CloudAzurePowerbidedicatedIntegration", "CloudAzureRediscacheIntegration", "CloudAzureServicebusIntegration", "CloudAzureServicefabricIntegration", "CloudAzureSqlIntegration", "CloudAzureSqlmanagedIntegration", "CloudAzureStorageIntegration", "CloudAzureVirtualmachineIntegration", "CloudAzureVirtualnetworksIntegration", "CloudAzureVmsIntegration", "CloudAzureVpngatewaysIntegration", "CloudBaseIntegration", "CloudBaseProvider", "CloudBillingIntegration", "CloudCloudfrontIntegration", "CloudCloudtrailIntegration", "CloudDynamodbIntegration", "CloudEbsIntegration", "CloudEc2Integration", "CloudEcsIntegration", "CloudEfsIntegration", "CloudElasticacheIntegration", "CloudElasticbeanstalkIntegration", "CloudElasticsearchIntegration", "CloudElbIntegration", "CloudEmrIntegration", "CloudGcpAppengineIntegration", "CloudGcpBigqueryIntegration", "CloudGcpDataflowIntegration", "CloudGcpDataprocIntegration", "CloudGcpDatastoreIntegration", "CloudGcpFirebasedatabaseIntegration", "CloudGcpFirebasehostingIntegration", "CloudGcpFirebasestorageIntegration", "CloudGcpFirestoreIntegration", "CloudGcpFunctionsIntegration", "CloudGcpInterconnectIntegration", "CloudGcpKubernetesIntegration", "CloudGcpLoadbalancingIntegration", "CloudGcpProvider", "CloudGcpPubsubIntegration", "CloudGcpRouterIntegration", "CloudGcpRunIntegration", "CloudGcpSpannerIntegration", "CloudGcpSqlIntegration", "CloudGcpStorageIntegration", "CloudGcpVmsIntegration", "CloudGcpVpcaccessIntegration", "CloudHealthIntegration", "CloudIamIntegration", "CloudIntegration", "CloudIotIntegration", "CloudKinesisFirehoseIntegration", "CloudKinesisIntegration", "CloudLambdaIntegration", "CloudLinkedAccount", "CloudProvider", "CloudRdsIntegration", "CloudRedshiftIntegration", "CloudRoute53Integration", "CloudS3Integration", "CloudService", "CloudSesIntegration", "CloudSnsIntegration", "CloudSqsIntegration", "CloudTrustedadvisorIntegration", "CloudVpcIntegration", "EpochSeconds", "Int", "String"},
		},
		"leveraging string replacer": {
			Types: []config.TypeConfig{},
			Mutations: []config.MutationConfig{{
				Name: "apiAccessCreateKeys",
			}},
			ExpectedNames: []string{"ApiAccessCreateIngestKeyInput", "ApiAccessCreateInput", "ApiAccessCreateKeyResponse", "ApiAccessCreateUserKeyInput", "ApiAccessIngestKey", "ApiAccessIngestKeyError", "ApiAccessIngestKeyErrorType", "ApiAccessIngestKeyType", "ApiAccessKey", "ApiAccessKeyError", "ApiAccessKeyType", "ApiAccessUserKey", "ApiAccessUserKeyError", "ApiAccessUserKeyErrorType", "ID", "Int", "String"},
		},
		"complicated cloud confirms complications": {
			Types: []config.TypeConfig{},
			Mutations: []config.MutationConfig{{
				Name: "cloudDisableIntegration",
			}},
			ExpectedNames: []string{"Boolean", "CloudAlbIntegration", "CloudApigatewayIntegration", "CloudAutoscalingIntegration", "CloudAwsAppsyncIntegration", "CloudAwsAthenaIntegration", "CloudAwsCognitoIntegration", "CloudAwsConnectIntegration", "CloudAwsDirectconnectIntegration", "CloudAwsDisableIntegrationsInput", "CloudAwsDocdbIntegration", "CloudAwsFsxIntegration", "CloudAwsGlueIntegration", "CloudAwsGovCloudProvider", "CloudAwsGovcloudDisableIntegrationsInput", "CloudAwsKinesisanalyticsIntegration", "CloudAwsMediaconvertIntegration", "CloudAwsMediapackagevodIntegration", "CloudAwsMqIntegration", "CloudAwsMskIntegration", "CloudAwsNeptuneIntegration", "CloudAwsProvider", "CloudAwsQldbIntegration", "CloudAwsRoute53resolverIntegration", "CloudAwsStatesIntegration", "CloudAwsTransitgatewayIntegration", "CloudAwsWafIntegration", "CloudAwsWafv2Integration", "CloudAwsXrayIntegration", "CloudAzureApimanagementIntegration", "CloudAzureAppgatewayIntegration", "CloudAzureAppserviceIntegration", "CloudAzureContainersIntegration", "CloudAzureCosmosdbIntegration", "CloudAzureCostmanagementIntegration", "CloudAzureDatafactoryIntegration", "CloudAzureDisableIntegrationsInput", "CloudAzureEventhubIntegration", "CloudAzureExpressrouteIntegration", "CloudAzureFirewallsIntegration", "CloudAzureFrontdoorIntegration", "CloudAzureFunctionsIntegration", "CloudAzureKeyvaultIntegration", "CloudAzureLoadbalancerIntegration", "CloudAzureLogicappsIntegration", "CloudAzureMachinelearningIntegration", "CloudAzureMariadbIntegration", "CloudAzureMysqlIntegration", "CloudAzurePostgresqlIntegration", "CloudAzurePowerbidedicatedIntegration", "CloudAzureRediscacheIntegration", "CloudAzureServicebusIntegration", "CloudAzureServicefabricIntegration", "CloudAzureSqlIntegration", "CloudAzureSqlmanagedIntegration", "CloudAzureStorageIntegration", "CloudAzureVirtualmachineIntegration", "CloudAzureVirtualnetworksIntegration", "CloudAzureVmsIntegration", "CloudAzureVpngatewaysIntegration", "CloudBaseIntegration", "CloudBaseProvider", "CloudBillingIntegration", "CloudCloudfrontIntegration", "CloudCloudtrailIntegration", "CloudDisableAccountIntegrationInput", "CloudDisableIntegrationPayload", "CloudDisableIntegrationsInput", "CloudDynamodbIntegration", "CloudEbsIntegration", "CloudEc2Integration", "CloudEcsIntegration", "CloudEfsIntegration", "CloudElasticacheIntegration", "CloudElasticbeanstalkIntegration", "CloudElasticsearchIntegration", "CloudElbIntegration", "CloudEmrIntegration", "CloudGcpAppengineIntegration", "CloudGcpBigqueryIntegration", "CloudGcpDataflowIntegration", "CloudGcpDataprocIntegration", "CloudGcpDatastoreIntegration", "CloudGcpDisableIntegrationsInput", "CloudGcpFirebasedatabaseIntegration", "CloudGcpFirebasehostingIntegration", "CloudGcpFirebasestorageIntegration", "CloudGcpFirestoreIntegration", "CloudGcpFunctionsIntegration", "CloudGcpInterconnectIntegration", "CloudGcpKubernetesIntegration", "CloudGcpLoadbalancingIntegration", "CloudGcpProvider", "CloudGcpPubsubIntegration", "CloudGcpRouterIntegration", "CloudGcpRunIntegration", "CloudGcpSpannerIntegration", "CloudGcpSqlIntegration", "CloudGcpStorageIntegration", "CloudGcpVmsIntegration", "CloudGcpVpcaccessIntegration", "CloudHealthIntegration", "CloudIamIntegration", "CloudIntegration", "CloudIntegrationMutationError", "CloudIotIntegration", "CloudKinesisFirehoseIntegration", "CloudKinesisIntegration", "CloudLambdaIntegration", "CloudLinkedAccount", "CloudProvider", "CloudRdsIntegration", "CloudRedshiftIntegration", "CloudRoute53Integration", "CloudS3Integration", "CloudService", "CloudSesIntegration", "CloudSnsIntegration", "CloudSqsIntegration", "CloudTrustedadvisorIntegration", "CloudVpcIntegration", "EpochSeconds", "Int", "String"},
		},
	}

	for _, tc := range cases {
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
