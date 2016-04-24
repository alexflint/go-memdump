package memdump

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func join(bufs ...[]byte) []byte {
	return bytes.Join(bufs, nil)
}

func TestDelimitedReader_Simple(t *testing.T) {
	data := join([]byte("abc"), delim, []byte("defggg"), delim)
	r := NewDelimitedReader(bytes.NewReader(data))

	seg, err := r.Next()
	assert.NoError(t, err)
	assert.Equal(t, "abc", string(seg))

	seg, err = r.Next()
	assert.NoError(t, err)
	assert.Equal(t, "defggg", string(seg))

	seg, err = r.Next()
	assert.Equal(t, io.EOF, err)
	assert.Nil(t, seg)
}

func TestDelimitedReader_Long(t *testing.T) {
	data := make([]byte, 32768)
	for i := range data {
		data[i] = 0
	}
	copy(data[len(data)-len(delim):], delim)
	r := NewDelimitedReader(bytes.NewReader(data))

	seg, err := r.Next()
	assert.NoError(t, err)
	assert.Len(t, seg, len(data)-len(delim))

	seg, err = r.Next()
	assert.Equal(t, io.EOF, err)
	assert.Nil(t, seg)
}

func TestDelimitedReader_SimpleThenEmpty(t *testing.T) {
	data := join([]byte("abc"), delim, delim)
	r := NewDelimitedReader(bytes.NewReader(data))

	seg, err := r.Next()
	assert.NoError(t, err)
	assert.Equal(t, "abc", string(seg))

	seg, err = r.Next()
	assert.NoError(t, err)
	assert.NotNil(t, seg)
	assert.Equal(t, "", string(seg))

	seg, err = r.Next()
	assert.Equal(t, io.EOF, err)
	assert.Nil(t, seg)
}

func TestDelimitedReader_Empty(t *testing.T) {
	r := NewDelimitedReader(bytes.NewReader(nil))
	seg, err := r.Next()
	assert.Equal(t, io.EOF, err)
	assert.Nil(t, seg)

	// same thing again
	seg, err = r.Next()
	assert.Equal(t, io.EOF, err)
	assert.Nil(t, seg)
}

func TestDelimitedReader_Unterminated(t *testing.T) {
	r := NewDelimitedReader(bytes.NewReader([]byte("abc")))
	_, err := r.Next()
	assert.Equal(t, ErrUnexpectedEOF, err)
}

func TestDelimitedReader_ReadAfterEOF(t *testing.T) {
	data := join([]byte("abc"), delim)
	r := NewDelimitedReader(bytes.NewReader(data))

	// first read: should not return error
	_, err := r.Next()
	assert.NoError(t, err)

	// second read: should return EOF
	_, err = r.Next()
	assert.Equal(t, io.EOF, err)

	// third read: should return EOF again
	_, err = r.Next()
	assert.Equal(t, io.EOF, err)
}
