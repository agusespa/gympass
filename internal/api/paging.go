package api

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
)

// List endpoints return at most limit items (default defaultPageSize) and a
// next_cursor while more remain; passing it back as cursor returns the next
// page. Cursors are opaque to clients.
const (
	defaultPageSize = 50
	maxPageSize     = 200
)

// paramError is a bad query parameter or tool argument.
type paramError string

func (e paramError) Error() string { return string(e) }

// pageSize parses limit: 0 (left out) means the default.
func pageSize(limit int) (int, error) {
	switch {
	case limit == 0:
		return defaultPageSize, nil
	case limit < 0 || limit > maxPageSize:
		return 0, paramError("limit must be 1 to " + strconv.Itoa(maxPageSize))
	}
	return limit, nil
}

// queryLimit reads the limit query parameter.
func queryLimit(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n == 0 {
		return 0, paramError("limit must be 1 to " + strconv.Itoa(maxPageSize))
	}
	return n, nil
}

func encodeCursor(parts ...string) string {
	b, _ := json.Marshal(parts)
	return base64.RawURLEncoding.EncodeToString(b)
}

// decodeCursor returns the n parts of a cursor made by encodeCursor, or nil
// for an empty one.
func decodeCursor(s string, n int) ([]string, error) {
	if s == "" {
		return nil, nil
	}
	var parts []string
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || json.Unmarshal(b, &parts) != nil || len(parts) != n {
		return nil, paramError("invalid cursor")
	}
	return parts, nil
}
