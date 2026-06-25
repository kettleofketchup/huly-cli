package huly

import (
	"crypto/rand"
	"encoding/hex"
	"sync/atomic"
	"time"
)

var refCounter uint32

// NewRef returns a Huly-compatible unique reference id: a hex string combining
// a millisecond timestamp, a process-random seed, and a monotonic counter.
func NewRef() string {
	var seed [8]byte
	_, _ = rand.Read(seed[:])
	ts := uint64(time.Now().UnixMilli())
	c := atomic.AddUint32(&refCounter, 1)
	buf := make([]byte, 0, 28)
	tsb := []byte{
		byte(ts >> 40), byte(ts >> 32), byte(ts >> 24),
		byte(ts >> 16), byte(ts >> 8), byte(ts),
	}
	buf = append(buf, tsb...)
	buf = append(buf, seed[:]...)
	buf = append(buf, byte(c>>8), byte(c))
	return hex.EncodeToString(buf)
}
