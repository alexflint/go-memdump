package memdump

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHomogenous_Struct(t *testing.T) {
	type T struct {
		X int
		Y string
	}
	src := []T{T{1, "s1"}, T{2, "s2"}, T{3, "s3"}}

	var b bytes.Buffer

	enc := NewEncoder(&b)
	for i, x := range src {
		t.Logf("encoding object %d", i)
		err := enc.Encode(&x)
		require.NoError(t, err)
	}

	dec := NewDecoder(&b)
	var dest []T
	for i := 0; ; i++ {
		t.Logf("decoding object %d", i)
		var x T
		err := dec.Decode(&x)
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		dest = append(dest, x)
	}

	assert.EqualValues(t, src, dest)
}
