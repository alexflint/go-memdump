package memdump

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocations(t *testing.T) {
	expected := locations{
		Main:     2,
		Pointers: []int64{3, 4},
	}

	var b bytes.Buffer

	err := encodeLocations(&b, &expected)
	require.NoError(t, err)

	var actual locations
	err = decodeLocations(&b, &actual)
	require.NoError(t, err)

	assert.Equal(t, expected, actual)
}

func TestRelocate_EmptyBuffer(t *testing.T) {
	var buf []byte
	_, err := relocate(buf, nil, 0, reflect.TypeOf(0))
	assert.Error(t, err)
}

func TestRelocate_MainOutOfBounds(t *testing.T) {
	buf := []byte{1, 2, 3}
	_, err := relocate(buf, nil, 100, reflect.TypeOf(0))
	assert.Error(t, err)
}

func TestRelocate_PointerOutOfBounds(t *testing.T) {
	buf := []byte{1, 2, 3}
	_, err := relocate(buf, []int64{100}, 0, reflect.TypeOf(0))
	assert.Error(t, err)
}

func BenchmarkLocations(b *testing.B) {
	in := locations{Pointers: make([]int64, 200000)}
	var out locations

	var buf bytes.Buffer
	err := encodeLocations(&buf, &in)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := bytes.NewBuffer(buf.Bytes())
		err = decodeLocations(r, &out)
		require.NoError(b, err)
	}
}
