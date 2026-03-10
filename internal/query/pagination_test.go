package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeDecode_Roundtrip(t *testing.T) {
	sortValues := []any{12.83, "sku_123"}
	cursor := EncodeCursor(sortValues)
	assert.NotEmpty(t, cursor)

	decoded := DecodeCursor(cursor)
	assert.Len(t, decoded, 2)
	assert.Equal(t, 12.83, decoded[0])
	assert.Equal(t, "sku_123", decoded[1])
}

func TestEncodeCursor_Empty(t *testing.T) {
	assert.Empty(t, EncodeCursor(nil))
	assert.Empty(t, EncodeCursor([]any{}))
}

func TestDecodeCursor_Empty(t *testing.T) {
	assert.Nil(t, DecodeCursor(""))
}

func TestDecodeCursor_Invalid(t *testing.T) {
	assert.Nil(t, DecodeCursor("not-valid-base64!!!"))
}

func TestDecodeCursor_InvalidJSON(t *testing.T) {
	// Valid base64 but invalid JSON inside.
	assert.Nil(t, DecodeCursor("bm90LWpzb24="))
}
