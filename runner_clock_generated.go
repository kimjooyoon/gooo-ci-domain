package cibudgetdomain

//gooo:generated:start id="urn:gooo:ci-domain:admit-runner-clock" kind="activity"
func AdmitRunnerClock(input0 int64, input1 int64) string {

	if input1 <= 0 || input1 > 9223372036854 {
		return "INVALID_DEADLINE"
	}
	if input0 < 0 {
		return "INVALID_COST"
	}
	if input0 >= input1 {
		return "DEADLINE_EXCEEDED"
	}
	return "READY"

}

//gooo:generated:end id="urn:gooo:ci-domain:admit-runner-clock" kind="activity"
