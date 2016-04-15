package memdump

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertSerialize(t *testing.T, obj interface{}) {
	var b bytes.Buffer
	enc := newMemEncoder(&b)
	ptrs, err := enc.Encode(obj)
	assert.NoError(t, err)
	f := footer{Pointers: ptrs}
	obj2, err := relocate(b.Bytes(), &f, reflect.TypeOf(obj).Elem())
	require.NoError(t, err)
	assert.EqualValues(t, obj, obj2)
}

func TestSerialize_Struct(t *testing.T) {
	type T struct {
		X int
		y uint8
	}
	obj := T{
		X: 3,
		y: 7,
	}
	assertSerialize(t, obj)
}
