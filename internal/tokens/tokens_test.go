package tokens

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateToken(t *testing.T) {
	before := time.Now()
	tok, err := GenerateToken(42, time.Hour, ScopeAuth)
	require.NoError(t, err)

	assert.Equal(t, 42, tok.UserID)
	assert.Equal(t, ScopeAuth, tok.Scope)
	assert.NotEmpty(t, tok.PlainText)

	// stored hash must be sha256(plaintext), never the plaintext itself
	want := sha256.Sum256([]byte(tok.PlainText))
	assert.Equal(t, want[:], tok.Hash)
	assert.Len(t, tok.Hash, 32)

	// expiry is now + ttl
	assert.WithinDuration(t, before.Add(time.Hour), tok.Expiry, time.Second)
}

func TestGenerateToken_Entropy(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		tok, err := GenerateToken(1, time.Hour, ScopeAuth)
		require.NoError(t, err)
		assert.False(t, seen[tok.PlainText], "duplicate token generated, entropy is broken")
		seen[tok.PlainText] = true
	}
}
