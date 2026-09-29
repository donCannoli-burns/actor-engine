package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
)

const (
	RuntimePrefix     = "run-"
	ObservationPrefix = "obs-"
)

func NewRuntimeID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate runtime id: %w", err)
	}
	return RuntimePrefix + hex.EncodeToString(b[:]), nil
}

func ObservationID(state map[string]string) string {
	keys := make([]string, 0, len(state))
	for key := range state {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	h := sha256.New()
	var length [8]byte
	write := func(value string) {
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		_, _ = h.Write(length[:])
		_, _ = h.Write([]byte(value))
	}
	for _, key := range keys {
		write(key)
		write(state[key])
	}
	return ObservationPrefix + hex.EncodeToString(h.Sum(nil))
}
