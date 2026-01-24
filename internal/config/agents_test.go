package config

import (
	"strings"
	"testing"
)

func TestValidateAgentsJSON_Valid(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "single agent with required fields",
			json: `{"reviewer": {"description": "Reviews code", "prompt": "You are a code reviewer"}}`,
		},
		{
			name: "single agent with all fields",
			json: `{"reviewer": {"description": "Reviews code", "prompt": "You are a code reviewer", "tools": ["Read", "Grep"], "model": "sonnet"}}`,
		},
		{
			name: "multiple agents",
			json: `{"reviewer": {"description": "Reviews code", "prompt": "You are a reviewer"}, "implementor": {"description": "Implements code", "prompt": "You are a developer"}}`,
		},
		{
			name: "empty tools array",
			json: `{"agent": {"description": "Test agent", "prompt": "Test prompt", "tools": []}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAgentsJSON(tt.json)
			if err != nil {
				t.Errorf("ValidateAgentsJSON() error = %v, want nil", err)
			}
		})
	}
}

func TestValidateAgentsJSON_InvalidSyntax(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "malformed JSON",
			json: `{invalid json}`,
		},
		{
			name: "unclosed brace",
			json: `{"agent": {"description": "test"`,
		},
		{
			name: "array instead of object",
			json: `["agent1", "agent2"]`,
		},
		{
			name: "empty string",
			json: ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAgentsJSON(tt.json)
			if err == nil {
				t.Error("ValidateAgentsJSON() error = nil, want error for invalid syntax")
			}
		})
	}
}

func TestValidateAgentsJSON_MissingFields(t *testing.T) {
	tests := []struct {
		name          string
		json          string
		wantErrSubstr string
	}{
		{
			name:          "missing description",
			json:          `{"agent": {"prompt": "You are a test"}}`,
			wantErrSubstr: "description",
		},
		{
			name:          "missing prompt",
			json:          `{"agent": {"description": "Test agent"}}`,
			wantErrSubstr: "prompt",
		},
		{
			name:          "empty description",
			json:          `{"agent": {"description": "", "prompt": "You are a test"}}`,
			wantErrSubstr: "description",
		},
		{
			name:          "empty prompt",
			json:          `{"agent": {"description": "Test agent", "prompt": ""}}`,
			wantErrSubstr: "prompt",
		},
		{
			name:          "empty object for agent",
			json:          `{"agent": {}}`,
			wantErrSubstr: "description",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAgentsJSON(tt.json)
			if err == nil {
				t.Error("ValidateAgentsJSON() error = nil, want error for missing field")
			}
			if !strings.Contains(err.Error(), tt.wantErrSubstr) {
				t.Errorf("ValidateAgentsJSON() error = %q, want error containing %q", err.Error(), tt.wantErrSubstr)
			}
		})
	}
}

func TestAgentsToJSON(t *testing.T) {
	tests := []struct {
		name   string
		agents map[string]Agent
		check  func(t *testing.T, json string)
	}{
		{
			name: "single agent with required fields",
			agents: map[string]Agent{
				"reviewer": {
					Description: "Reviews code",
					Prompt:      "You are a code reviewer",
				},
			},
			check: func(t *testing.T, json string) {
				if !strings.Contains(json, `"reviewer"`) {
					t.Error("JSON should contain agent name 'reviewer'")
				}
				if !strings.Contains(json, `"description":"Reviews code"`) {
					t.Error("JSON should contain description")
				}
				if !strings.Contains(json, `"prompt":"You are a code reviewer"`) {
					t.Error("JSON should contain prompt")
				}
			},
		},
		{
			name: "agent with optional fields",
			agents: map[string]Agent{
				"builder": {
					Description: "Builds code",
					Prompt:      "You build things",
					Tools:       []string{"Bash", "Read"},
					Model:       "opus",
				},
			},
			check: func(t *testing.T, json string) {
				if !strings.Contains(json, `"tools":["Bash","Read"]`) {
					t.Error("JSON should contain tools array")
				}
				if !strings.Contains(json, `"model":"opus"`) {
					t.Error("JSON should contain model")
				}
			},
		},
		{
			name:   "empty agents map",
			agents: map[string]Agent{},
			check: func(t *testing.T, json string) {
				if json != "{}" {
					t.Errorf("Empty agents should produce '{}', got %q", json)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			json, err := AgentsToJSON(tt.agents)
			if err != nil {
				t.Fatalf("AgentsToJSON() error = %v", err)
			}
			tt.check(t, json)

			// Validate the output is valid JSON
			if err := ValidateAgentsJSON(json); err != nil && len(tt.agents) > 0 {
				t.Errorf("AgentsToJSON() produced invalid JSON: %v", err)
			}
		})
	}
}
