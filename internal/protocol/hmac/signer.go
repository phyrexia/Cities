// Package hmac provides HMAC-SHA256 message signing and verification
// for the Cities game protocol. Every client message is signed with the
// player's unique secret key, and the coordinator rejects unsigned or
// tampered messages.
package hmac

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	// TimestampTolerance is the max clock skew allowed between client and server.
	TimestampTolerance = 2 * time.Minute
	// SecretKeyLength is the byte length of generated secret keys.
	SecretKeyLength = 32
)

// GenerateSecretKey creates a new cryptographically random secret key.
// Called once per player during registration; stored server-side and delivered to client.
func GenerateSecretKey() (string, error) {
	b := make([]byte, SecretKeyLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secret key: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Sign creates an HMAC-SHA256 signature for a message payload.
// message: the raw JSON bytes of the game message
// timestamp: Unix timestamp (seconds) included in the signed data
// secretKey: player's hex-encoded secret key
func Sign(message []byte, timestamp int64, secretKey string) (string, error) {
	key, err := hex.DecodeString(secretKey)
	if err != nil {
		return "", fmt.Errorf("decode secret key: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(fmt.Sprintf("%d:", timestamp)))
	mac.Write(message)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// Verify checks that a message signature is valid and not replayed.
// Returns an error if the signature is wrong, expired, or the timestamp is out of range.
func Verify(message []byte, timestamp int64, signature, secretKey string) error {
	// Check timestamp freshness
	age := time.Since(time.Unix(timestamp, 0))
	if age < -TimestampTolerance || age > TimestampTolerance {
		return fmt.Errorf("message timestamp out of tolerance (age: %s)", age)
	}

	expected, err := Sign(message, timestamp, secretKey)
	if err != nil {
		return fmt.Errorf("compute expected signature: %w", err)
	}

	// Constant-time comparison to prevent timing attacks
	expectedBytes, _ := hex.DecodeString(expected)
	gotBytes, err := hex.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	if !hmac.Equal(expectedBytes, gotBytes) {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

// SignedMessage is the wire format for all client→server messages.
type SignedMessage struct {
	PlayerID  string `json:"player_id"`
	Timestamp int64  `json:"ts"`          // Unix seconds
	Signature string `json:"sig"`         // HMAC-SHA256 hex
	Payload   []byte `json:"payload"`     // raw JSON of the actual message
}
