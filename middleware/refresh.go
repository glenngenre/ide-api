package middleware

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"strconv"
	"time"
)

func accessTTL() time.Duration {
	if v := os.Getenv("JWT_TTL_MINUTES"); v != "" {
		if m, err := strconv.Atoi(v); err == nil && m > 0 {
			return time.Duration(m) * time.Minute
		}
	}
	return 15 * time.Minute
}

func RefreshTTL() time.Duration {
	if v := os.Getenv("REFRESH_TTL_DAYS"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d > 0 {
			return time.Duration(d) * 24 * time.Hour
		}
	}
	return 30 * 24 * time.Hour
}

func NewRefreshToken() (plain string, hash []byte, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", nil, err
	}
	plain = base64.RawURLEncoding.EncodeToString(buf)
	return plain, HashRefreshToken(plain), nil
}

func HashRefreshToken(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))
	return sum[:]
}

func NewFamilyID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
