// Package worktree provides functionality for managing git worktrees in orbital.
package worktree

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
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
// Falls back to appending a numeric suffix if base combinations are taken.
func GenerateUniqueName(excluded []string) string {
	excludeSet := make(map[string]bool)
	for _, name := range excluded {
		excludeSet[name] = true
	}

	// With 50x50=2500 combinations and trying 50 times, we have good coverage
	// even with many existing worktrees. P(collision) for n existing is ~(n/2500)^50
	for i := 0; i < 50; i++ {
		name := GenerateName()
		if !excludeSet[name] {
			return name
		}
	}

	// Fallback: append timestamp-based suffix for guaranteed uniqueness
	// This handles the edge case where we have many collisions
	base := GenerateName()
	suffix := time.Now().UnixNano() % 10000 // Last 4 digits of nanosecond timestamp
	for attempts := 0; attempts < 100; attempts++ {
		name := fmt.Sprintf("%s-%d", base, suffix+int64(attempts))
		if !excludeSet[name] {
			return name
		}
	}

	// Final fallback: use full timestamp (extremely unlikely to reach here)
	return fmt.Sprintf("%s-%d", base, time.Now().UnixNano())
}

// randomInt returns a cryptographically secure random integer in [0, max).
func randomInt(max int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		// Fallback to time-based pseudo-random if crypto/rand fails.
		// This is less secure but provides reasonable distribution for name generation.
		return int(time.Now().UnixNano() % int64(max))
	}
	return int(n.Int64())
}
