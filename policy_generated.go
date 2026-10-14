package cibudgetdomain

//gooo:generated:start id="urn:gooo:ci-domain:admit-plan" kind="activity"
func AdmitPlan(input0 int64, input1 int64, input2 int64, input3 bool, input4 bool) string {

	if !input3 {
		return "IMPACT_UNRESOLVED"
	}
	if !input4 {
		return "COST_UNRESOLVED"
	}
	if input2 <= 0 || input2 > 180000 {
		return "INVALID_DEADLINE"
	}
	if input0 < 0 || input1 < 0 {
		return "INVALID_COST"
	}
	if input1 > input2 {
		return "OVER_BUDGET"
	}
	if input0 > input2-input1 {
		return "OVER_BUDGET"
	}
	return "READY"

}

//gooo:generated:end id="urn:gooo:ci-domain:admit-plan" kind="activity"
