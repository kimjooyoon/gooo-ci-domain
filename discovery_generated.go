package cibudgetdomain

//gooo:generated:start id="urn:gooo:ci-domain:admit-discovery" kind="activity"
func AdmitDiscovery(input0 int64, input1 int64, input2 bool, input3 bool) string {

	if !input2 {
		return "IMPACT_UNRESOLVED"
	}
	if !input3 {
		return "SOURCE_ALREADY_OBSERVED"
	}
	if input0 <= 0 || input0 > 9223372036854 {
		return "INVALID_DEADLINE"
	}
	if input1 < 0 {
		return "INVALID_COST"
	}
	if input1 >= input0 {
		return "DEADLINE_EXCEEDED"
	}
	return "MEASURE_NEW_COHORT"

}

//gooo:generated:end id="urn:gooo:ci-domain:admit-discovery" kind="activity"
