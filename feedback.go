package cibudgetdomain

import (
	"fmt"
	"math"
)

// Calibration keeps incomplete runs as censored observations. A completed
// successor must resolve that cost before this context becomes predictable.
func Calibrate(key string, observations []Observation) (Estimate, error) {
	result, ids := Estimate{CostKey: key}, map[string]bool{}
	if key == "" || len(observations) > 256 {
		return result, fmt.Errorf("cost context or observation population differs")
	}
	for _, observation := range observations {
		if observation.CostKey != key {
			continue
		}
		if observation.ID == "" || ids[observation.ID] || observation.ElapsedMS < 0 ||
			observation.PredictedMS < 0 || observation.ResolutionMS < 0 ||
			observation.ElapsedMS > math.MaxInt64-observation.ResolutionMS {
			return result, fmt.Errorf("timing origin or value differs")
		}
		ids[observation.ID] = true
		result.Origins = append(result.Origins, observation.ID)
		if !observation.Completed {
			result.Censored++
			result.CensoredFloorMS = max(result.CensoredFloorMS, observation.ElapsedMS)
			continue
		}
		result.Samples++
		upper := observation.ElapsedMS + observation.ResolutionMS
		result.ObservedMaxMS = max(result.ObservedMaxMS, upper)
		if observation.PredictionState != "unavailable" {
			result.ResidualMS = max(result.ResidualMS, upper-observation.PredictedMS)
		}
	}
	if result.ObservedMaxMS > math.MaxInt64-result.ResidualMS {
		return result, fmt.Errorf("forecast exceeds integer representation")
	}
	result.ForecastMS = ReserveCost(result.ObservedMaxMS, result.ResidualMS)
	return result, nil
}
