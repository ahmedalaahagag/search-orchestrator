package query

import (
	"encoding/base64"
	"encoding/json"
)

// EncodeCursor encodes sort values from the last hit into an opaque cursor string.
func EncodeCursor(sortValues []any) string {
	if len(sortValues) == 0 {
		return ""
	}
	data, err := json.Marshal(sortValues)
	if err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(data)
}

// DecodeCursor decodes an opaque cursor string back into sort values.
func DecodeCursor(cursor string) []any {
	if cursor == "" {
		return nil
	}
	data, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return nil
	}
	var values []any
	if err := json.Unmarshal(data, &values); err != nil {
		return nil
	}
	return values
}
