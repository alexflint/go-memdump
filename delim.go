package structbuffer

import (
	"errors"
	"io"
	"reflect"
)

// delim is used to recognize the end of the memory dump
var delim = []byte{
	130, 4, 133, 49, 108, 178, 125, 95,
	35, 126, 41, 129, 229, 48, 6, 94,
	69, 20, 194, 236, 79, 156, 67, 100,
	239, 152, 149, 93, 91, 56, 8, 183,
}

// ErrBufferTooSmall is returned by delimetedReader if the input buffer
// is smaller than the length of the delimeter
var ErrBufferTooSmall = errors.New(
	"cannot read into buffer of size less than 32 bytes (due to delim)")

// ErrUnexpectedEOF is returned by delimetedReader if EOF is reached
// before finding a delimeter
var ErrUnexpectedEOF = errors.New(
	"got EOF before finding the delimeter")

// delimetedReader reads until the delim above is reached
type delimitedReader struct {
	r       io.Reader
	buf     []byte
	atdelim bool
	eof     bool
}

// newDelimetedReader creates a delimited reader
func newDelimitedReader(r io.Reader) *delimitedReader {
	return &delimitedReader{
		r: r,
	}
}

// Read extracts bytes from the underlying reader and returns EOF
// when the delim is reached.
func (r *delimitedReader) Read(dest []byte) (int, error) {
	if len(dest) < len(delim) {
		return 0, ErrBufferTooSmall
	}

	// check whether we are already at delim
	if r.atdelim {
		return 0, io.EOF
	}

	// first copy from buf
	var nbuf int
	if len(r.buf) > 0 {
		nbuf = copy(dest, r.buf)
		if nbuf == len(r.buf) {
			r.buf = r.buf[:0]
		} else {
			r.buf = r.buf[nbuf:]
		}
	}

	// fill the rest of dest
	var nread int
	if nbuf < len(dest) && !r.eof {
		var err error
		nread, err = r.r.Read(dest[nbuf:])
		if err == io.EOF {
			r.eof = true
		} else if err != nil {
			return nbuf + nread, err
		}
	}

	// should be impossible
	if nread == 0 && nbuf == 0 && !r.eof {
		panic("read zero bytes from both buffer and reader")
	}

	// look for the delimeter
	var errout error
	var nout, nskip, state int
	for i, b := range dest[:nbuf+nread] {
		if b != delim[state] {
			state = 0
		}
		// do not use "else" here because we updated state above
		if b == delim[state] {
			state++
			if state == len(delim) {
				r.atdelim = true
				errout = io.EOF
				nskip = len(delim)
				break
			}
		} else {
			nout = i + 1
		}
	}

	// if we got EOF but no delimeter then we have an error
	if r.eof && !r.atdelim {
		return nout, ErrUnexpectedEOF
	}

	// update buffer
	r.buf = append(r.buf[:0], dest[nout+nskip:nbuf+nread]...)
	return nout, errout
}

// Next proceeds to the next segment and returns true if there is another
// segment to extract.
func (r *delimitedReader) Next() bool {
	if r.eof && len(r.buf) == 0 {
		return false
	}
	r.atdelim = false
	return true
}

// Encoder writes objects to a stream
type Encoder struct {
	w io.Writer
}

// NewEncoder creates an encoder that writes to the provided writer
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// Encode adds an object to the output stream. The object should
// be a pointer to the object you wish to encode. To encode a
// pointer to the stream, pass a double pointer.
func (e *Encoder) Encode(obj interface{}) error {
	val := reflect.ValueOf(obj)
	_ = val
	// TODO
	return nil
}
