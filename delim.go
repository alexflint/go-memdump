package memdump

import (
	"errors"
	"io"
	"log"
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
			} else {
				return nil, ErrUnexpectedEOF
			}
		} else if err != nil {
			return nil, err
		}
	}
}

// delimetedReader reads until the delim above is reached
type delimitedReader struct {
	r       io.Reader
	buf     []byte
	atdelim bool
	hasread bool // true if any bytes have been read for the current segment
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
	// if we are already at EOF then we are done
	if r.atdelim {
		return 0, io.EOF
	}

	// check the size of the buffer
	if len(dest) < len(delim) {
		return 0, ErrBufferTooSmall
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
		if nread > 0 {
			r.hasread = true
		}
	}

	if nread == 0 && nbuf == 0 {
		if !r.eof {
			// should be impossible
			panic("read zero bytes from both buffer and reader")
		}
		if r.hasread {
			return 0, ErrUnexpectedEOF
		}
		return 0, io.EOF
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
	r.buf = append(r.buf, dest[nout+nskip:nbuf+nread]...)
	log.Printf("read %d fresh bytes, got %d from buf, outputting %d, skipped %d, state=%d, len(buf)=%d", nread, nbuf, nout, nskip, state, len(r.buf))
	return nout, errout
}

// Next proceeds to the next segment and returns true if there is another
// segment to extract.
func (r *delimitedReader) Next() {
	r.atdelim = false
	r.hasread = false
}

// EOF returns true if all segments have been consumed
func (r *delimitedReader) EOF() bool {
	return len(r.buf) == 0 && r.eof
}
