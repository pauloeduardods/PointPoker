package service

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Field length limits, in runes.
const (
	MaxDisplayNameLen = 50
	MaxRoomNameLen    = 100
	MaxStoryTitleLen  = 200
)

// NormalizeCode canonicalizes a room code: codes are case-insensitive and
// stored upper-case.
func NormalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// cleanText trims s and checks that it has between 1 and max runes.
func cleanText(field, s string, max int) (string, error) {
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n < 1 || n > max {
		return "", newError(ErrInvalid, "%s must be between 1 and %d characters", field, max)
	}
	return s, nil
}

// isUUID reports whether s is a well-formed UUID. Used to reject malformed
// identifiers before they reach the database.
func isUUID(s string) bool {
	return uuid.Validate(s) == nil
}
