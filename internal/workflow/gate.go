package workflow

import "strings"

// GateResult represents the result of a gate check.
type GateResult int

const (
	// GateNotFound means no gate signal was found in the output.
	GateNotFound GateResult = iota

	// GatePassed means the gate passed.
	GatePassed

	// GateFailed means the gate failed.
	GateFailed
)

// GatePassTag is the tag that signals a gate passed.
const GatePassTag = "<gate>PASS</gate>"

// GateFailTag is the tag that signals a gate failed.
const GateFailTag = "<gate>FAIL</gate>"

// CheckGate examines output for gate pass/fail signals.
// Returns the gate result.
func CheckGate(output string) GateResult {
	hasPass := strings.Contains(output, GatePassTag)
	hasFail := strings.Contains(output, GateFailTag)

	// If both are present, the last one wins
	if hasPass && hasFail {
		passIndex := strings.LastIndex(output, GatePassTag)
		failIndex := strings.LastIndex(output, GateFailTag)
		if passIndex > failIndex {
			return GatePassed
		}
		return GateFailed
	}

	if hasPass {
		return GatePassed
	}
	if hasFail {
		return GateFailed
	}

	return GateNotFound
}

// String returns a human-readable representation of the gate result.
func (r GateResult) String() string {
	switch r {
	case GatePassed:
		return "PASS"
	case GateFailed:
		return "FAIL"
	default:
		return "NOT_FOUND"
	}
}
