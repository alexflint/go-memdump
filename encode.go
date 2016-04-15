package memdump

import (
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
)

const version int32 = 1

type footer struct {
	Pointers []int // Pointers contains the offset of each pointer
	Main     int   // Main contains the offset of the primary object
}

// Encoder writes memdumps to the provided writer
type Encoder struct {
	w           io.Writer
	hasprotocol bool
}

// NewEncoder creates an encoder that writes memdumps to the provided writer
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w: w,
	}
}

// Encode writes a memdump of the provided object to output. You must pass a
// pointer to the object you wish to encode. To encode a pointer, pass a
// double-pointer.
func (e *Encoder) Encode(obj interface{}) error {
	// write a protocol version number
	if !e.hasprotocol {
		err := binary.Write(e.w, binary.LittleEndian, version)
		if err != nil {
			return fmt.Errorf("error writing protocol: %v", err)
		}
		e.hasprotocol = true
	}

	// first segment: write the object data
	mem := newMemEncoder(e.w)
	ptrs, err := mem.Encode(obj)
	if err != nil {
		return fmt.Errorf("error writing data segment: %v", err)
	}

	// write delimiter
	_, err = e.w.Write(delim)
	if err != nil {
		return fmt.Errorf("error writing delimiter: %v", err)
	}

	// second segment: write the metadata
	gob := gob.NewEncoder(e.w)
	err = gob.Encode(footer{
		Pointers: ptrs,
	})
	if err != nil {
		return fmt.Errorf("error writing footer: %v", err)
	}

	// write delimiter
	_, err = e.w.Write(delim)
	if err != nil {
		return fmt.Errorf("error writing delimiter: %v", err)
	}
	return nil
}

// Decoder reads memdumps from the provided reader
type Decoder struct {
	r           io.Reader
	dr          *delimitedReader
	hasprotocol bool
}

// NewDecoder creates a decoder that reads memdumps
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r:  r,
		dr: newDelimitedReader(r),
	}
}

// Decode reads an object of the specified type from the input.
// The object passed to Decode must be a pointer to the type
// was originally passed to Encode().
func (d *Decoder) Decode(dest interface{}) error {
	ptr, err := d.DecodePtr(reflect.TypeOf(dest).Elem())
	if err != nil {
		return err
	}
	reflect.ValueOf(dest).Elem().Set(reflect.ValueOf(ptr).Elem())
	return nil
}

// DecodePtr reads an object of the specified type from the input
// and returns a pointer to it. The provided type must be the result
// of calling reflect.TypeOf(x) where x is the object originally
// passed to Encode(). The return valoue will be of type *x
func (d *Decoder) DecodePtr(typ reflect.Type) (interface{}, error) {
	// read protocol
	if !d.hasprotocol {
		var protocol int32
		err := binary.Read(d.r, binary.LittleEndian, &protocol)
		if err != nil {
			return nil, fmt.Errorf("error reading protocol: %v", err)
		}
		if protocol != version {
			return nil, fmt.Errorf("invalid protocol %d", protocol)
		}
		d.hasprotocol = true
	}

	// first segment: read the memory buffer
	membuf, err := ioutil.ReadAll(d.dr)
	if len(membuf) == 0 && err == io.EOF {
		return nil, io.EOF
	}
	if err != nil {
		return nil, fmt.Errorf("error reading data segment: %v", err)
	}
	d.dr.Next()

	// second segment: read the footer
	var f footer
	dec := gob.NewDecoder(d.dr)
	err = dec.Decode(&f)
	if err == io.EOF {
		return nil, errors.New("footer was missing")
	}
	if err != nil {
		return nil, fmt.Errorf("error decoding footer: %v", err)
	}
	d.dr.Next()

	// relocate the data
	return relocate(membuf, &f, typ)
}
