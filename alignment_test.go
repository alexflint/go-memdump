package memdump

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type byteAndInt struct {
	b *byte // should be aligned to a 1-byte boundary
	i *int  // should be aligned to an 8-byte boundary
}

func newByte(b byte) *byte {
	return &b
}

func newInt(i int) *int {
	return &i
}

func assertAligned(t *testing.T, v reflect.Value) {
	addr := v.UnsafeAddr()

	assert.Zero(t, addr%uintptr(v.Type().Align()),
		"alignment of %v was off by %d", v.Type(), addr%uintptr(v.Type().Align()))
}

func TestAlignment(t *testing.T) {
	in := byteAndInt{
		b: newByte(3),
		i: newInt(4),
	}

	var buf bytes.Buffer
	err := Encode(&buf, &in)
	require.NoError(t, err)

	var out *byteAndInt
	err = Decode(&buf, &out)
	require.NoError(t, err)

	assertAligned(t, reflect.ValueOf(out.b).Elem())
	assertAligned(t, reflect.ValueOf(out.i).Elem())
}
