package memdump

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testEncodeDecode(t *testing.T, src interface{}, dest interface{}) {
	var b bytes.Buffer

	enc := NewHeterogeneousEncoder(&b)
	err := enc.Encode(src)
	require.NoError(t, err)

	dec := NewHeterogeneousDecoder(&b)
	err = dec.Decode(dest)
	require.NoError(t, err)
}

func TestHeterogeneous_Int(t *testing.T) {
	var dest int
	var src int = 3
	testEncodeDecode(t, &src, &dest)
	assert.EqualValues(t, src, dest)
}

func TestHeterogeneous_String(t *testing.T) {
	var dest string
	src := "abc"
	testEncodeDecode(t, &src, &dest)
	assert.EqualValues(t, src, dest)
}

func TestHeterogeneous_Slice(t *testing.T) {
	var dest []int16
	src := []int16{5, 4, 3}
	testEncodeDecode(t, &src, &dest)
	assert.EqualValues(t, src, dest)
}

func TestHeterogeneous_Large(t *testing.T) {
	type T struct {
		A string
		B int
	}
	src := make([]T, 1000)
	for i := range src {
		src[i].A = strings.Repeat("123", 100)
		src[i].B = 123
	}
	var dest []T
	testEncodeDecode(t, &src, &dest)
	assert.EqualValues(t, src, dest)
}

func TestHeterogeneous_Struct(t *testing.T) {
	type U struct {
		W string
		X *string
	}
	type T struct {
		U
		A int
		B byte
		C []U
		D []*U
		E *[]U
		F *U
		G *T
	}

	s := "hello"
	u := U{W: s, X: &s}
	src := T{
		U: u,
		A: 5,
		B: 6,
		C: []U{u},
		D: []*U{&u},
		F: &u,
	}
	src.E = &src.C
	src.G = &src

	var dest T
	testEncodeDecode(t, &src, &dest)
	assert.EqualValues(t, src, dest)
}

func TestHeterogeneous_Incompatible(t *testing.T) {
	type U struct {
		X int
		Y string
	}
	type V struct {
		X string
		Y int
	}

	src := U{3, "abc"}
	var dest V

	var b bytes.Buffer

	enc := NewHeterogeneousEncoder(&b)
	err := enc.Encode(&src)
	require.NoError(t, err)

	dec := NewHeterogeneousDecoder(&b)
	err = dec.Decode(&dest)
	assert.Equal(t, ErrIncompatibleLayout, err)
}

func TestHeterogeneousEncodeUnsupportedTypes(t *testing.T) {
	var buf bytes.Buffer
	enc := NewHeterogeneousEncoder(&buf)
	assert.Panics(t, func() {
		enc.Encode(&map[string]int{})
	})
	assert.Panics(t, func() {
		enc.Encode(func() {})
	})
	assert.Panics(t, func() {
		enc.Encode(make(chan int))
	})
	assert.Panics(t, func() {
		var x interface{} = "abc"
		enc.Encode(&x)
	})
}
