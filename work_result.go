package cibudgetdomain

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// An observed command outcome remains readable when its final persistence fails.
// The nonzero exit and original error still report the incomplete operation.
func writeWorkResult(stdout, stderr io.Writer, result WorkReceipt, runErr error) int {
	if runErr == nil || result.Observation != nil {
		runErr = errors.Join(runErr, json.NewEncoder(stdout).Encode(result))
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
