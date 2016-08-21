package memdump

import (
	"bytes"
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

	assert.Equal(t, expected, actual)
}

func BenchmarkLocations(b *testing.B) {
	in := locations{Pointers: make([]int64, 200000)}
	var out locations

	var buf bytes.Buffer
	encodeLocations(&buf, &in)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := bytes.NewBuffer(buf.Bytes())
		//ioutil.ReadAll(r)
		//_ = out
		decodeLocations(r, &out)
	}
}
