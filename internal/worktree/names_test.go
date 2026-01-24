package worktree

import (
	"regexp"
	"testing"
)

func TestGenerateName(t *testing.T) {
	t.Run("returns adjective-animal format", func(t *testing.T) {
		name := GenerateName()

		// Should match pattern: lowercase-letters-hyphen-lowercase-letters
		pattern := regexp.MustCompile(`^[a-z]+-[a-z]+$`)
		if !pattern.MatchString(name) {
			t.Errorf("GenerateName() = %q; want format adjective-animal", name)
		}
	})

	t.Run("returns different values across multiple calls", func(t *testing.T) {
		seen := make(map[string]bool)
		uniqueCount := 0

		// Generate 20 names and count unique ones
		for i := 0; i < 20; i++ {
			name := GenerateName()
			if !seen[name] {
				seen[name] = true
				uniqueCount++
			}
		}

		// With 2500 combinations, we should get at least 10 unique names in 20 tries
		if uniqueCount < 10 {
			t.Errorf("GenerateName() produced only %d unique names in 20 calls; want at least 10", uniqueCount)
		}
	})

	t.Run("uses valid adjectives and animals", func(t *testing.T) {
		adjSet := make(map[string]bool)
		for _, adj := range adjectives {
			adjSet[adj] = true
		}
		animalSet := make(map[string]bool)
		for _, animal := range animals {
			animalSet[animal] = true
		}

		// Generate several names and verify components
		for i := 0; i < 50; i++ {
			name := GenerateName()
			parts := splitName(name)
			if len(parts) != 2 {
				t.Fatalf("GenerateName() = %q; expected two parts separated by hyphen", name)
			}

			if !adjSet[parts[0]] {
				t.Errorf("GenerateName() adjective %q not in adjectives list", parts[0])
			}
			if !animalSet[parts[1]] {
				t.Errorf("GenerateName() animal %q not in animals list", parts[1])
			}
		}
	})
}

func TestGenerateUniqueName(t *testing.T) {
	t.Run("returns valid name with nil exclusions", func(t *testing.T) {
		name := GenerateUniqueName(nil)

		pattern := regexp.MustCompile(`^[a-z]+-[a-z]+$`)
		if !pattern.MatchString(name) {
			t.Errorf("GenerateUniqueName(nil) = %q; want format adjective-animal", name)
		}
	})

	t.Run("returns valid name with empty exclusions", func(t *testing.T) {
		name := GenerateUniqueName([]string{})

		pattern := regexp.MustCompile(`^[a-z]+-[a-z]+$`)
		if !pattern.MatchString(name) {
			t.Errorf("GenerateUniqueName([]) = %q; want format adjective-animal", name)
		}
	})

	t.Run("returns name not in exclusion list", func(t *testing.T) {
		excluded := []string{"swift-falcon", "calm-otter", "bold-eagle"}

		for i := 0; i < 20; i++ {
			name := GenerateUniqueName(excluded)
			for _, ex := range excluded {
				if name == ex {
					t.Errorf("GenerateUniqueName() = %q; should not be in exclusion list", name)
				}
			}
		}
	})

	t.Run("returns name with suffix when all combinations excluded", func(t *testing.T) {
		// Create exclusion list with all possible combinations
		var allCombinations []string
		for _, adj := range adjectives {
			for _, animal := range animals {
				allCombinations = append(allCombinations, adj+"-"+animal)
			}
		}

		name := GenerateUniqueName(allCombinations)

		// Should have a numeric suffix
		pattern := regexp.MustCompile(`^[a-z]+-[a-z]+-\d+$`)
		if !pattern.MatchString(name) {
			t.Errorf("GenerateUniqueName(all) = %q; want format adjective-animal-N", name)
		}
	})

	t.Run("finds name after several collisions", func(t *testing.T) {
		// Exclude most but not all combinations
		var mostCombinations []string
		count := 0
		for _, adj := range adjectives {
			for _, animal := range animals {
				if count < 2400 { // Leave 100 combinations available
					mostCombinations = append(mostCombinations, adj+"-"+animal)
					count++
				}
			}
		}

		name := GenerateUniqueName(mostCombinations)

		// Should find an available name without suffix
		excluded := make(map[string]bool)
		for _, n := range mostCombinations {
			excluded[n] = true
		}

		// Name should either not be excluded OR have a suffix
		if excluded[name] {
			t.Errorf("GenerateUniqueName() = %q; should not be in exclusion list", name)
		}
	})
}

func TestAdjectivesAndAnimalsLists(t *testing.T) {
	t.Run("has 50 adjectives", func(t *testing.T) {
		if len(adjectives) != 50 {
			t.Errorf("adjectives has %d items; want 50", len(adjectives))
		}
	})

	t.Run("has 50 animals", func(t *testing.T) {
		if len(animals) != 50 {
			t.Errorf("animals has %d items; want 50", len(animals))
		}
	})

	t.Run("adjectives are unique", func(t *testing.T) {
		seen := make(map[string]bool)
		for _, adj := range adjectives {
			if seen[adj] {
				t.Errorf("duplicate adjective: %q", adj)
			}
			seen[adj] = true
		}
	})

	t.Run("animals are unique", func(t *testing.T) {
		seen := make(map[string]bool)
		for _, animal := range animals {
			if seen[animal] {
				t.Errorf("duplicate animal: %q", animal)
			}
			seen[animal] = true
		}
	})

	t.Run("adjectives are lowercase", func(t *testing.T) {
		for _, adj := range adjectives {
			for _, r := range adj {
				if r < 'a' || r > 'z' {
					t.Errorf("adjective %q contains non-lowercase character", adj)
					break
				}
			}
		}
	})

	t.Run("animals are lowercase", func(t *testing.T) {
		for _, animal := range animals {
			for _, r := range animal {
				if r < 'a' || r > 'z' {
					t.Errorf("animal %q contains non-lowercase character", animal)
					break
				}
			}
		}
	})
}

// splitName splits an adjective-animal name into its components.
func splitName(name string) []string {
	for i := 0; i < len(name); i++ {
		if name[i] == '-' {
			return []string{name[:i], name[i+1:]}
		}
	}
	return []string{name}
}
