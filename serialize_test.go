package memdump

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func assertSerialize(t *testing.T, obj interface{}) {
	var b bytes.Buffer
	enc := newMemEncoder(&b)
	err := enc.Encode(obj)
	assert.NoError(t, err)
	obj2 := Relocate(b.Bytes(), enc.ptrLocs, reflect.TypeOf(obj).Elem())
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
	assertSerialize(t, &obj)
}
