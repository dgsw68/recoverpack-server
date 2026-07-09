package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"
)

const tokenLifetime = 24 * time.Hour

type tokenClaims struct {
	Subject   string `json:"sub"`
	Email     string `json:"email"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func authSecret() []byte {
	if secret := os.Getenv("AUTH_SECRET"); len(secret) >= 32 {
		return []byte(secret)
	}
	// Local development fallback only. Production must set AUTH_SECRET.
	return []byte("recoverpack-local-development-secret-change-me")
}

func CreateToken(userID, email string) (string, error) {
	now := time.Now()
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, err := json.Marshal(tokenClaims{
		Subject: userID, Email: email,
		IssuedAt: now.Unix(), ExpiresAt: now.Add(tokenLifetime).Unix(),
	})
	if err != nil {
		return "", err
	}
	unsigned := encodeSegment(header) + "." + encodeSegment(payload)
	return unsigned + "." + sign(unsigned), nil
}

func ParseToken(token string) (*tokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token")
	}
	unsigned := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(sign(unsigned)), []byte(parts[2])) {
		return nil, errors.New("invalid token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid token payload")
	}
	var claims tokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, errors.New("invalid token claims")
	}
	if claims.Subject == "" || claims.ExpiresAt <= time.Now().Unix() {
		return nil, errors.New("expired token")
	}
	return &claims, nil
}

func sign(unsigned string) string {
	mac := hmac.New(sha256.New, authSecret())
	_, _ = mac.Write([]byte(unsigned))
	return encodeSegment(mac.Sum(nil))
}

func encodeSegment(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}
