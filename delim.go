package memdump

import (
	"errors"
	"io"
)

// delim is used to recognize the end of the memory dump
var delim = []byte{
	130, 14, 133, 49, 108, 178, 125, 95,
	35, 126, 41, 129, 229, 48, 16, 94,
}

// ErrBufferTooSmall is returned by delimetedReader if the input buffer
// is smaller than the length of the delimeter
var ErrBufferTooSmall = errors.New(
	"cannot read into buffer of size less than 16 bytes (due to delim)")

// ErrUnexpectedEOF is returned by delimetedReader if EOF is reached
// before finding a delimeter
var ErrUnexpectedEOF = errors.New(
	"got EOF before finding the delimeter")

// DelimitedReader reads delimited segments
type DelimitedReader struct {
	r     io.Reader
	buf   []byte
	begin int
	end   int
}

// NewDelimitedReader creates a reader for delimited segments
func NewDelimitedReader(r io.Reader) *DelimitedReader {
	return &DelimitedReader{
		r: r,
	}
}

// Next returns the next segment, or (nil, io.EOF) if there are no more segments.
// The data is only valid until the next call to Next().
func (r *DelimitedReader) Next() ([]byte, error) {
	var state, offset int
	for {
		// look for the next delimiter
		for i, b := range r.buf[r.begin+offset : r.end] {
			if b != delim[state] {
				state = 0
			}
			// do not use "else" here because we updated state above
			if b == delim[state] {
				state++
				if state == len(delim) {
					out := r.buf[r.begin : r.begin+offset+i-len(delim)+1]
					r.begin = r.begin + offset + i + 1
					return out, nil
				}
			}
		}
		offset = r.end - r.begin

		// allocate a larger buffer
		if r.buf == nil {
			r.buf = make([]byte, 16384)
		} else {
			var newbuf []byte
			newbuf = make([]byte, 4*len(r.buf))
			copy(newbuf, r.buf[r.begin:r.end])
			r.end -= r.begin
			r.begin = 0
			r.buf = newbuf
		}

		// fill the rest of the buffer
		n, err := r.r.Read(r.buf[r.end:])
		r.end += n

		// check for exit conditions
		if n == 0 && err == io.EOF {
			if offset == 0 {
				return nil, io.EOF
			}
			return nil, ErrUnexpectedEOF
		} else if err != nil {
			return nil, err
		}
	}
}
