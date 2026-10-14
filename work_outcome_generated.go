package cibudgetdomain

//gooo:generated:start id="urn:gooo:ci-domain:classify-work-outcome" kind="activity"
func ClassifyWorkOutcome(input0 bool, input1 int64, input2 bool) string {

	if input0 {
		return "DEADLINE_EXCEEDED"
	}
	if !input2 {
		return "ORIGIN_UNAVAILABLE"
	}
	if input1 != 0 {
		return "VERIFY_FAILED"
	}
	return "COMMAND_COMPLETED"

}

//gooo:generated:end id="urn:gooo:ci-domain:classify-work-outcome" kind="activity"
