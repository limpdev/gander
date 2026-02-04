package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"time"
)

const (
	AUTH_SESSION_COOKIE_NAME     = "session_token"
	AUTH_RATE_LIMIT_WINDOW       = 5 * time.Minute
	AUTH_RATE_LIMIT_MAX_ATTEMPTS = 5
)
const (
	AUTH_TOKEN_SECRET_LENGTH  = 32
	AUTH_USERNAME_HASH_LENGTH = 32
	AUTH_SECRET_KEY_LENGTH    = AUTH_TOKEN_SECRET_LENGTH + AUTH_USERNAME_HASH_LENGTH
	AUTH_TIMESTAMP_LENGTH     = 4 // uint32
	AUTH_TOKEN_DATA_LENGTH    = AUTH_USERNAME_HASH_LENGTH + AUTH_TIMESTAMP_LENGTH
)

// How long the token will be valid for
const (
	AUTH_TOKEN_VALID_PERIOD = 14 * 24 * time.Hour // 14 days
	// How long the token has left before it should be regenerated
	AUTH_TOKEN_REGEN_BEFORE = 7 * 24 * time.Hour // 7 days
)

type FailedAuthAttempt struct {
	Attempts int
	First    time.Time
}

func GenerateSessionToken(username string, secret []byte, now time.Time) (string, error) {
	if len(secret) != AUTH_SECRET_KEY_LENGTH {
		return "", fmt.Errorf("secret key length is not %d bytes", AUTH_SECRET_KEY_LENGTH)
	}
	usernameHash, err := ComputeUsernameHash(username, secret)
	if err != nil {
		return "", err
	}
	data := make([]byte, AUTH_TOKEN_DATA_LENGTH)
	copy(data, usernameHash)
	expires := now.Add(AUTH_TOKEN_VALID_PERIOD).Unix()
	binary.LittleEndian.PutUint32(data[AUTH_USERNAME_HASH_LENGTH:], uint32(expires))
	h := hmac.New(sha256.New, secret[0:AUTH_TOKEN_SECRET_LENGTH])
	h.Write(data)
	signature := h.Sum(nil)
	encodedToken := base64.StdEncoding.EncodeToString(append(data, signature...))
	// encodedToken ends up being (hashed username + expiration timestamp + signature) encoded as base64
	return encodedToken, nil
}
func ComputeUsernameHash(username string, secret []byte) ([]byte, error) {
	if len(secret) != AUTH_SECRET_KEY_LENGTH {
		return nil, fmt.Errorf("secret key length is not %d bytes", AUTH_SECRET_KEY_LENGTH)
	}
	h := hmac.New(sha256.New, secret[AUTH_TOKEN_SECRET_LENGTH:])
	h.Write([]byte(username))
	return h.Sum(nil), nil
}
func VerifySessionToken(token string, secretBytes []byte, now time.Time) ([]byte, bool, error) {
	tokenBytes, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, false, err
	}
	if len(tokenBytes) != AUTH_TOKEN_DATA_LENGTH+32 {
		return nil, false, fmt.Errorf("token length is invalid")
	}
	if len(secretBytes) != AUTH_SECRET_KEY_LENGTH {
		return nil, false, fmt.Errorf("secret key length is not %d bytes", AUTH_SECRET_KEY_LENGTH)
	}
	usernameHashBytes := tokenBytes[0:AUTH_USERNAME_HASH_LENGTH]
	timestampBytes := tokenBytes[AUTH_USERNAME_HASH_LENGTH : AUTH_USERNAME_HASH_LENGTH+AUTH_TIMESTAMP_LENGTH]
	providedSignatureBytes := tokenBytes[AUTH_TOKEN_DATA_LENGTH:]
	h := hmac.New(sha256.New, secretBytes[0:32])
	h.Write(tokenBytes[0:AUTH_TOKEN_DATA_LENGTH])
	expectedSignatureBytes := h.Sum(nil)
	if !hmac.Equal(expectedSignatureBytes, providedSignatureBytes) {
		return nil, false, fmt.Errorf("signature does not match")
	}
	expiresTimestamp := int64(binary.LittleEndian.Uint32(timestampBytes))
	if now.Unix() > expiresTimestamp {
		return nil, false, fmt.Errorf("token has expired")
	}
	return usernameHashBytes,
		// True if the token should be regenerated
		time.Unix(expiresTimestamp, 0).Add(-AUTH_TOKEN_REGEN_BEFORE).Before(now),
		nil
}
func MakeAuthSecretKey(length int) (string, error) {
	key := make([]byte, length)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
