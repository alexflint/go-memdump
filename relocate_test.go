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
		Pointers: []int{3, 4},
	}

	var b bytes.Buffer

	err := encodeLocations(&b, &expected)
	require.NoError(t, err)

	var actual locations
	err = decodeLocations(&b, &actual)

	assert.Equal(t, expected, actual)
}
