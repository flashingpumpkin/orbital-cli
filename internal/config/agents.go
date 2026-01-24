// Package config provides configuration management for orbit.
package config

import (
	"encoding/json"
	"fmt"
)

// Agent represents a custom agent definition for TOML config files.
type Agent struct {
	Description string   `toml:"description" json:"description"`
	Prompt      string   `toml:"prompt" json:"prompt"`
	Tools       []string `toml:"tools,omitempty" json:"tools,omitempty"`
	Model       string   `toml:"model,omitempty" json:"model,omitempty"`
}

// AgentDefinition represents the JSON format expected by Claude CLI --agents flag.
type AgentDefinition struct {
	Description string   `json:"description"`
	Prompt      string   `json:"prompt"`
	Tools       []string `json:"tools,omitempty"`
	Model       string   `json:"model,omitempty"`
}

// ValidateAgentsJSON validates that a JSON string is properly formatted
// and contains valid agent definitions with required fields.
func ValidateAgentsJSON(jsonStr string) error {
	if jsonStr == "" {
		return fmt.Errorf("agents JSON cannot be empty")
	}

	var agents map[string]AgentDefinition
	if err := json.Unmarshal([]byte(jsonStr), &agents); err != nil {
		return fmt.Errorf("invalid JSON syntax: %w", err)
	}

	for name, agent := range agents {
		if agent.Description == "" {
			return fmt.Errorf("agent %q is missing required field: description", name)
		}
		if agent.Prompt == "" {
			return fmt.Errorf("agent %q is missing required field: prompt", name)
		}
	}

	return nil
}

// AgentsToJSON converts a map of Agent structs (from TOML config) to JSON string
// suitable for passing to Claude CLI --agents flag.
func AgentsToJSON(agents map[string]Agent) (string, error) {
	if len(agents) == 0 {
		return "{}", nil
	}

	// Convert to the JSON format expected by Claude CLI
	result := make(map[string]AgentDefinition)
	for name, agent := range agents {
		def := AgentDefinition{
			Description: agent.Description,
			Prompt:      agent.Prompt,
		}
		if len(agent.Tools) > 0 {
			def.Tools = agent.Tools
		}
		if agent.Model != "" {
			def.Model = agent.Model
		}
		result[name] = def
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal agents to JSON: %w", err)
	}

	return string(jsonBytes), nil
}
