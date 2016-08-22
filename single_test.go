package memdump

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSingle(t *testing.T) {
	type T struct {
		X  int
		Y  string
		Ts []*T
	}
	src := T{
		X: 123,
		Y: "abc",
		Ts: []*T{
			{4, "x", nil},
			{5, "y", nil},
		},
	}

	var b bytes.Buffer

	err := Encode(&b, &src)
	require.NoError(t, err)

	var dest *T
	err = Decode(&b, &dest)
	require.NoError(t, err)

	assert.EqualValues(t, src, *dest)
}

func TestEncode_PanicsForNonPointer(t *testing.T) {
	var x struct{}
	var b bytes.Buffer
	assert.Panics(t, func() { Encode(&b, x) })
}

func TestDecode_PanicsForNonPointerToPointer(t *testing.T) {
	var x struct{}
	var b bytes.Buffer
	assert.Panics(t, func() { Decode(&b, x) })
	assert.Panics(t, func() { Decode(&b, &x) })
}
