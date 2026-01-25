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

// DefaultAgents contains built-in agent definitions that are always available.
// These are merged with user-defined agents, with user agents taking precedence.
var DefaultAgents = map[string]Agent{
	"general-purpose": {
		Description: "General-purpose agent for researching complex questions, searching for code, and executing multi-step tasks",
		Prompt:      "You are a general-purpose agent that helps with research, code exploration, and multi-step tasks. Use available tools to gather information and complete the task thoroughly.",
	},
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
// User agents are merged with DefaultAgents, with user agents taking precedence.
func AgentsToJSON(agents map[string]Agent) (string, error) {
	// Merge defaults with user agents (user takes precedence)
	merged := MergeAgents(DefaultAgents, agents)

	if len(merged) == 0 {
		return "{}", nil
	}

	// Convert to the JSON format expected by Claude CLI
	result := make(map[string]AgentDefinition)
	for name, agent := range merged {
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

// MergeAgents merges two agent maps, with the second map taking precedence.
// This is used to combine DefaultAgents with user-defined agents.
func MergeAgents(base, override map[string]Agent) map[string]Agent {
	result := make(map[string]Agent)

	// Copy base agents
	for name, agent := range base {
		result[name] = agent
	}

	// Override with user agents
	for name, agent := range override {
		result[name] = agent
	}

	return result
}

// GetEffectiveAgents returns the merged default and user agents as JSON.
// If no user agents are provided, returns only default agents.
// If userAgentsJSON is provided and valid, it's merged with defaults.
func GetEffectiveAgents(userAgentsJSON string) (string, error) {
	if userAgentsJSON == "" {
		// No user agents, return defaults
		return AgentsToJSON(nil)
	}

	// Validate user agents JSON
	if err := ValidateAgentsJSON(userAgentsJSON); err != nil {
		return "", err
	}

	// Parse user agents and merge
	var userAgents map[string]Agent
	if err := json.Unmarshal([]byte(userAgentsJSON), &userAgents); err != nil {
		return "", fmt.Errorf("failed to parse user agents: %w", err)
	}

	return AgentsToJSON(userAgents)
}
