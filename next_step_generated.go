package cibudgetdomain

//gooo:generated:start id="urn:gooo:ci-domain:next-step" kind="activity"
func NextStep(input string) string {

	if input == "READY" {
		return "EXECUTE_SCOPED_PLAN"
	}
	if input == "IMPACT_UNRESOLVED" {
		return "RESOLVE_INPUT_ORIGIN"
	}
	if input == "COST_UNRESOLVED" {
		return "COLLECT_COST_ORIGIN"
	}
	if input == "OVER_BUDGET" {
		return "REFINE_REQUIRED_WORK"
	}
	if input == "DEADLINE_EXCEEDED" {
		return "RECALIBRATE_CENSORED_COST"
	}
	if input == "VERIFY_FAILED" {
		return "REPAIR_SOURCE"
	}
	if input == "VERIFIED_SCOPED" {
		return "RECORD_ORIGIN"
	}
	if input == "VERIFIED_DISCOVERY" {
		return "RECORD_ORIGIN"
	}
	return "REPAIR_PLAN_CONTRACT"

}

//gooo:generated:end id="urn:gooo:ci-domain:next-step" kind="activity"
