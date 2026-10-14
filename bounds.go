package cibudgetdomain

import (
	"fmt"
	"math"
)

// These are conditional lower bounds under the calibrated unit cost estimates,
// not bounds on an unobserved future execution time.
func workBounds(graph Graph, required []bool, costs []int64, workers int) (WorkBounds, error) {
	result := WorkBounds{}
	done, ends := make([]bool, len(required)), make([]int64, len(required))
	remaining := 0
	for index, selected := range required {
		if !selected {
			done[index] = true
		} else {
			remaining++
		}
	}
	for remaining > 0 {
		index, start := nextReadyUnit(graph, required, done, ends)
		if index < 0 {
			return result, fmt.Errorf("work bound has an unresolved cycle")
		}
		if costs[index] > math.MaxInt64-start || costs[index] > math.MaxInt64-result.SerialMS {
			return result, fmt.Errorf("work bound exceeds integer representation")
		}
		ends[index] = start + costs[index]
		result.SerialMS += costs[index]
		result.CriticalPathMS = max(result.CriticalPathMS, ends[index])
		done[index] = true
		remaining--
	}
	result.WorkerLowerMS = result.SerialMS / int64(workers)
	if result.SerialMS%int64(workers) != 0 {
		result.WorkerLowerMS++
	}
	result.MinimumMS = max(result.CriticalPathMS, result.WorkerLowerMS)
	return result, nil
}
