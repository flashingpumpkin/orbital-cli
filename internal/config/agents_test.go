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
				// Should also contain default agents
				if !strings.Contains(json, `"general-purpose"`) {
					t.Error("JSON should contain default agent 'general-purpose'")
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
			name:   "nil agents map includes defaults",
			agents: nil,
			check: func(t *testing.T, json string) {
				if !strings.Contains(json, `"general-purpose"`) {
					t.Errorf("Nil agents should include default 'general-purpose', got %q", json)
				}
			},
		},
		{
			name:   "empty agents map includes defaults",
			agents: map[string]Agent{},
			check: func(t *testing.T, json string) {
				if !strings.Contains(json, `"general-purpose"`) {
					t.Errorf("Empty agents should include default 'general-purpose', got %q", json)
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
			if err := ValidateAgentsJSON(json); err != nil {
				t.Errorf("AgentsToJSON() produced invalid JSON: %v", err)
			}
		})
	}
}

func TestDefaultAgents(t *testing.T) {
	t.Run("contains general-purpose agent", func(t *testing.T) {
		agent, ok := DefaultAgents["general-purpose"]
		if !ok {
			t.Fatal("DefaultAgents should contain 'general-purpose'")
		}
		if agent.Description == "" {
			t.Error("general-purpose agent should have a description")
		}
		if agent.Prompt == "" {
			t.Error("general-purpose agent should have a prompt")
		}
	})
}

func TestMergeAgents(t *testing.T) {
	tests := []struct {
		name     string
		base     map[string]Agent
		override map[string]Agent
		check    func(t *testing.T, result map[string]Agent)
	}{
		{
			name: "override takes precedence",
			base: map[string]Agent{
				"agent1": {Description: "base", Prompt: "base prompt"},
			},
			override: map[string]Agent{
				"agent1": {Description: "override", Prompt: "override prompt"},
			},
			check: func(t *testing.T, result map[string]Agent) {
				if result["agent1"].Description != "override" {
					t.Error("override should take precedence")
				}
			},
		},
		{
			name: "both agents preserved when no conflict",
			base: map[string]Agent{
				"agent1": {Description: "one", Prompt: "one"},
			},
			override: map[string]Agent{
				"agent2": {Description: "two", Prompt: "two"},
			},
			check: func(t *testing.T, result map[string]Agent) {
				if len(result) != 2 {
					t.Errorf("expected 2 agents, got %d", len(result))
				}
				if _, ok := result["agent1"]; !ok {
					t.Error("agent1 should be present")
				}
				if _, ok := result["agent2"]; !ok {
					t.Error("agent2 should be present")
				}
			},
		},
		{
			name:     "nil maps handled",
			base:     nil,
			override: nil,
			check: func(t *testing.T, result map[string]Agent) {
				if len(result) != 0 {
					t.Errorf("expected empty map, got %d agents", len(result))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeAgents(tt.base, tt.override)
			tt.check(t, result)
		})
	}
}

func TestGetEffectiveAgents(t *testing.T) {
	t.Run("empty user agents returns defaults", func(t *testing.T) {
		json, err := GetEffectiveAgents("")
		if err != nil {
			t.Fatalf("GetEffectiveAgents() error = %v", err)
		}
		if !strings.Contains(json, `"general-purpose"`) {
			t.Error("should contain default general-purpose agent")
		}
	})

	t.Run("user agents merged with defaults", func(t *testing.T) {
		userJSON := `{"custom": {"description": "Custom agent", "prompt": "You are custom"}}`
		json, err := GetEffectiveAgents(userJSON)
		if err != nil {
			t.Fatalf("GetEffectiveAgents() error = %v", err)
		}
		if !strings.Contains(json, `"general-purpose"`) {
			t.Error("should contain default general-purpose agent")
		}
		if !strings.Contains(json, `"custom"`) {
			t.Error("should contain user custom agent")
		}
	})

	t.Run("user agent overrides default", func(t *testing.T) {
		userJSON := `{"general-purpose": {"description": "Custom GP", "prompt": "Custom prompt"}}`
		json, err := GetEffectiveAgents(userJSON)
		if err != nil {
			t.Fatalf("GetEffectiveAgents() error = %v", err)
		}
		if !strings.Contains(json, `"Custom GP"`) {
			t.Error("user agent should override default")
		}
	})

	t.Run("invalid user JSON returns error", func(t *testing.T) {
		_, err := GetEffectiveAgents(`{invalid`)
		if err == nil {
			t.Error("should return error for invalid JSON")
		}
	})
}
