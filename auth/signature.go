// Package auth implements the client request-signing scheme described in
// docs/ARCHITECTURE.md §10: every write is signed by a per-device Ed25519
// keypair, with a timestamp+nonce to bound and detect replay.
package auth

import (
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
)

// Canonical builds the exact byte string that gets signed by the client and
// re-verified by the server. Both sides must produce identical bytes.
func Canonical(method, path string, timestamp string, nonce string, body []byte) []byte {
	bodyHash := sha256.Sum256(body)
	s := fmt.Sprintf("%s\n%s\n%s\n%s\n%x", method, path, timestamp, nonce, bodyHash)
	return []byte(s)
}

func Sign(privateKey ed25519.PrivateKey, canonical []byte) []byte {
	return ed25519.Sign(privateKey, canonical)
}

func Verify(publicKey ed25519.PublicKey, canonical []byte, signature []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(publicKey, canonical, signature)
}
