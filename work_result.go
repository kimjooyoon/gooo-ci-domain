package cibudgetdomain

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// An observed command outcome remains readable when its final persistence fails.
// The nonzero exit and original error still report the incomplete operation.
func writeWorkResult(stdout, stderr io.Writer, result WorkReceipt, runErr error) int {
	if runErr == nil || result.Observation != nil {
		runErr = errors.Join(runErr, json.NewEncoder(stdout).Encode(result))
		if result.Decision != "COMMAND_COMPLETED" {
			prediction := "unknown"
			if result.PredictionMS != nil {
				prediction = strconv.FormatInt(*result.PredictionMS, 10)
			}
			fmt.Fprintf(stderr, "work decision=%q next_operation=%q admission=%q prediction_ms=%s scope_elapsed_ms=%d deadline_ms=%d unit_elapsed_ms=%d completed=%t exit_code=%d\n",
				result.Decision, result.NextOperation, result.Admission, prediction, result.ScopeElapsedMS,
				result.DeadlineMS, result.UnitElapsedMS, result.Completed, result.ExitCode)
		}
	}
	if runErr != nil {
		fmt.Fprintln(stderr, runErr)
		return 1
	}
	if result.Decision != "COMMAND_COMPLETED" {
		return 1
	}
	return 0
}
