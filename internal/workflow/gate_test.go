package workflow

import "testing"

func TestCheckGate(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   GateResult
	}{
		{
			name:   "pass tag",
			output: "Review complete. All looks good.\n<gate>PASS</gate>",
			want:   GatePassed,
		},
		{
			name:   "fail tag",
			output: "Issues found:\n- Missing tests\n<gate>FAIL</gate>",
			want:   GateFailed,
		},
		{
			name:   "no tag",
			output: "Some random output without gate tags",
			want:   GateNotFound,
		},
		{
			name:   "pass then fail - fail wins",
			output: "<gate>PASS</gate> wait no <gate>FAIL</gate>",
			want:   GateFailed,
		},
		{
			name:   "fail then pass - pass wins",
			output: "<gate>FAIL</gate> actually it's fine <gate>PASS</gate>",
			want:   GatePassed,
		},
		{
			name:   "empty output",
			output: "",
			want:   GateNotFound,
		},
		{
			name:   "partial tag",
			output: "<gate>PASS",
			want:   GateNotFound,
		},
		{
			name:   "case sensitive - wrong case",
			output: "<gate>pass</gate>",
			want:   GateNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckGate(tt.output)
			if got != tt.want {
				t.Errorf("CheckGate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGateResult_String(t *testing.T) {
	tests := []struct {
		result GateResult
		want   string
	}{
		{GatePassed, "PASS"},
		{GateFailed, "FAIL"},
		{GateNotFound, "NOT_FOUND"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.result.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
