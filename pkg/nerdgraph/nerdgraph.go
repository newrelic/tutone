package nerdgraph

// Create a baseline NRQL condition.
func AlertsNrqlConditionBaselineCreate(
	accountID int,
	condition AlertsNrqlConditionBaselineInput,
	policyID int,
) (*AlertsNrqlBaselineCondition, error) {

	// TODO Do we have a "Response{}" object that we can refer to yet?
	resp := alertsNrqlConditionBaselineCreateResponse{}
	vars := map[string]interface{}{
		"accountId": accountID,
		"condition": condition,
		"policyId":  policyID,
	}

	if err := a.client.NerdGraphQuery(alertsNrqlConditionBaselineCreateMutation, vars, &resp); err != nil {
		return nil, err
	}

	return &resp.AlertsNrqlBaselineCondition, nil
}

type alertsNrqlConditionBaselineCreateResponse struct {
	AlertsNrqlConditionBaselineCreate AlertsNrqlBaselineCondition `json:"alertsNrqlConditionBaselineCreate"`
}

const alertsNrqlConditionBaselineCreateMutation = `
	mutation AlertsNrqlConditionBaselineCreate(
      $accountId: Int!
      $condition: AlertsNrqlConditionBaselineInput!
      $policyId: ID!
  ){
		alertsNrqlConditionBaselineCreate(
      accountId: $accountID,
      condition: $condition,
      policyId: $policyID,
    ) { ` + alertsNrqlConditionBaselineCreateFields + ` } }
`
