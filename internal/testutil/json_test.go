package testutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mrz1836/go-enformion/internal/testutil"
)

func TestEsc(t *testing.T) {
	assert.Equal(t, `"\u0061\\"`, testutil.Esc(`"~u0061~~"`))
	assert.Equal(t, "plain", testutil.Esc("plain"))
}

func TestTokenWalk(t *testing.T) {
	tests := []struct {
		name       string
		data       string
		valid, dup bool
	}{
		{name: "object", data: `{"a":1,"b":"a"}`, valid: true},
		{name: "scalar", data: `1`, valid: true},
		{name: "empty array", data: `[]`, valid: true},
		{name: "duplicate", data: `{"a":1,"a":2}`, valid: true, dup: true},
		{name: "duplicate spelled with an escape", data: testutil.Esc(`{"a":1,"~u0061":2}`), valid: true, dup: true},
		{name: "same key in sibling objects", data: `[{"a":1},{"a":2}]`, valid: true},
		{name: "keys after a nested object", data: `{"a":{"b":1},"c":[2],"d":3}`, valid: true},
		{name: "duplicate in a nested object", data: `{"a":[1,{"b":1,"b":2}]}`, valid: true, dup: true},
		{name: "trailing data", data: `{"a":1} {}`},
		{name: "truncated", data: `{"a":`},
		{name: "invalid UTF-8", data: "\"\xff\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, dup := testutil.TokenWalk([]byte(tt.data))
			assert.Equal(t, tt.valid, valid, "valid")
			assert.Equal(t, tt.dup, dup, "dup")
		})
	}
}
