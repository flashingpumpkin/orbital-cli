// Package worktree provides functionality for managing git worktrees in orbital.
package worktree

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

var adjectives = []string{
	"agile", "bold", "brave", "bright", "brisk",
	"calm", "clever", "cool", "cosmic", "crisp",
	"dapper", "daring", "deft", "eager", "epic",
	"fair", "fancy", "fast", "fierce", "fleet",
	"gentle", "glad", "golden", "grand", "great",
	"happy", "hardy", "hasty", "honest", "humble",
	"jolly", "keen", "kind", "lively", "lucky",
	"merry", "mighty", "modest", "noble", "plucky",
	"proud", "quick", "quiet", "rapid", "ready",
	"sharp", "sleek", "smart", "swift", "wise",
}

var animals = []string{
	"badger", "bear", "beaver", "bison", "bobcat",
	"cheetah", "condor", "cougar", "coyote", "crane",
	"deer", "dolphin", "eagle", "elk", "falcon",
	"ferret", "finch", "fox", "gazelle", "gecko",
	"hawk", "heron", "horse", "husky", "jaguar",
	"kestrel", "koala", "lemur", "leopard", "lion",
	"lynx", "marten", "moose", "otter", "owl",
	"panther", "parrot", "pelican", "puma", "raven",
	"salmon", "seal", "shark", "sparrow", "stork",
	"swift", "tiger", "turtle", "viper", "wolf",
}

// GenerateName returns a random adjective-animal combination in the format "adjective-animal".
func GenerateName() string {
	adj := adjectives[randomInt(len(adjectives))]
	animal := animals[randomInt(len(animals))]
	return fmt.Sprintf("%s-%s", adj, animal)
}

// GenerateUniqueName returns a name not in the excluded list.
// Falls back to appending a numeric suffix if all base combinations are taken.
func GenerateUniqueName(excluded []string) string {
	excludeSet := make(map[string]bool)
	for _, name := range excluded {
		excludeSet[name] = true
	}

	// Try up to 10 times to find an unused name
	for i := 0; i < 10; i++ {
		name := GenerateName()
		if !excludeSet[name] {
			return name
		}
	}

	// Fallback: append suffix
	base := GenerateName()
	for suffix := 2; suffix <= 100; suffix++ {
		name := fmt.Sprintf("%s-%d", base, suffix)
		if !excludeSet[name] {
			return name
		}
	}

	// Give up and return a name with a high suffix
	return fmt.Sprintf("%s-%d", base, 101)
}

// randomInt returns a cryptographically secure random integer in [0, max).
func randomInt(max int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		// Fallback to a deterministic value in the unlikely event of crypto/rand failure
		return 0
	}
	return int(n.Int64())
}
