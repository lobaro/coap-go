package coap

import (
	"math/rand"
	"sync"
	"time"
)

type TokenGenerator interface {
	NextToken() []byte
}

type RandomTokenGenerator struct {
	lastTokenSeq uint8      // Sequence counter
	rand         *rand.Rand // Random source for token generation

	mu sync.Mutex
}

func NewRandomTokenGenerator() TokenGenerator {
	return &RandomTokenGenerator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (t *RandomTokenGenerator) NextToken() []byte {
	// It's critical to not get the same token twice,
	// since we identify our interactions by the token
	t.mu.Lock()
	defer t.mu.Unlock()
	tok := make([]byte, 4)
	t.rand.Read(tok)
	t.lastTokenSeq++
	tok[0] = t.lastTokenSeq
	return tok
}

// Mainly used for tests, uses 1 Byte tokens that simply count up
type CountingTokenGenerator struct {
	lastTokenSeq uint8 // Sequence counter
	mu           sync.Mutex
}

func NewCountingTokenGenerator() TokenGenerator {
	return &CountingTokenGenerator{}
}

func (t *CountingTokenGenerator) NextToken() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	tok := make([]byte, 1)
	t.lastTokenSeq++
	tok[0] = t.lastTokenSeq
	return tok
}
