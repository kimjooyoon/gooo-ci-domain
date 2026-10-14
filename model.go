package cibudgetdomain

type Unit struct {
	ID, Kind, CostKey string
	Dependencies      []int
}

type Graph struct {
	SourceDigest string
	Units        []Unit
	Changed      []int
	Unresolved   []string
}

// CostKey identifies a unit's definition and runtime profile. Timings are
// prediction inputs; they never substitute for a current verification result.
type Observation struct {
	ID, CostKey                 string
	SourceDigest, Unit, Outcome string
	PredictionState             string
	SourceTree, Profile         string
	InputOrigins                []InputOrigin
	InputLogState               string
	OutputDigest                string
	ElapsedMS, PredictedMS      int64
	ResolutionMS                int64
	Completed                   bool
}

type Estimate struct {
	CostKey                   string
	Samples, Censored         int
	ObservedMaxMS, ResidualMS int64
	CensoredFloorMS           int64
	ForecastMS                int64
	Origins                   []string
}

type Slot struct {
	Unit, Worker int
	StartMS      int64
	EndMS        int64
}

type Plan struct {
	Decision                         string
	SourceDigest                     string
	ChangedUnits, AffectedUnits      int
	RequiredTests, RequiredExecution []string
	ForecastWorkMS                   *int64
	OverheadMS                       int64
	DeadlineMS                       int64
	Workers                          int
	Slots                            []Slot
	Unresolved                       []string
	CostBounds                       *WorkBounds
	UnknownCosts                     []CostCause
	TimingOrigins                    []string
}

type CostCause struct{ Unit, CostKey, Reason string }

type WorkBounds struct{ SerialMS, CriticalPathMS, WorkerLowerMS, MinimumMS int64 }

type InputOrigin struct{ Operation, Path string }
