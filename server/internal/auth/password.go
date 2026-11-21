package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const passwordIterations = 600_000

var rawURL = base64.RawURLEncoding.Strict()

func NormalizePassword(password string) (string, bool) {
	if !utf8.ValidString(password) {
		return "", false
	}
	password = norm.NFC.String(password)
	count := utf8.RuneCountInString(password)
	return password, count >= 10 && count <= 128
}

func hashPassword(password string, iterations int) (string, error) {
	var salt [16]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt[:], iterations, 32)
	if err != nil {
		return "", err
	}
	return "pbkdf2-sha256$" + strconv.Itoa(iterations) + "$" + rawURL.EncodeToString(salt[:]) + "$" + rawURL.EncodeToString(key), nil
}

func verifyPassword(password, encoded string) (bool, int, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false, 0, errors.New("invalid stored password hash")
	}
	iterations, err := strconv.Atoi(parts[1])
	salt, saltErr := rawURL.DecodeString(parts[2])
	key, keyErr := rawURL.DecodeString(parts[3])
	if err != nil || iterations <= 0 || saltErr != nil || keyErr != nil || len(salt) != 16 || len(key) != 32 {
		return false, 0, errors.New("invalid stored password hash")
	}
	actual, err := pbkdf2.Key(sha256.New, password, salt, iterations, 32)
	if err != nil {
		return false, 0, err
	}
	return subtle.ConstantTimeCompare(actual, key) == 1, iterations, nil
}

func newSecret() (string, error) {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return rawURL.EncodeToString(random[:]), nil
}

func ValidSecret(value string) bool {
	if len(value) != rawURL.EncodedLen(32) {
		return false
	}
	raw, err := rawURL.DecodeString(value)
	return err == nil && len(raw) == 32
}

func hashSecret(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
