package memdump

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"reflect"
)

type heterogeneousFooter struct {
	Pointers   []int64 // Pointers contains the offset of each pointer
	Main       int64   // Main contains the offset of the primary object
	Descriptor descriptor
}

// HeterogeneousEncoder writes memdumps to the provided writer
type HeterogeneousEncoder struct {
	w           io.Writer
	hasprotocol bool
}

// NewHeterogeneousEncoder creates an HeterogeneousEncoder that writes memdumps to the provided writer
func NewHeterogeneousEncoder(w io.Writer) *HeterogeneousEncoder {
	return &HeterogeneousEncoder{
		w: w,
	}
}

// Encode writes a memdump of the provided object to output. You must pass a
// pointer to the object you wish to encode. To encode a pointer, pass a
// double-pointer.
func (e *HeterogeneousEncoder) Encode(obj interface{}) error {
	t := reflect.TypeOf(obj)
	if t.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("expected a pointer but got %T", obj))
	}

	// write a protocol heterogeneousProtocol number
	if !e.hasprotocol {
		err := binary.Write(e.w, binary.LittleEndian, heterogeneousProtocol)
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
	err = gob.Encode(heterogeneousFooter{
		Pointers:   ptrs,
		Descriptor: describe(t.Elem()),
	})
	if err != nil {
		return fmt.Errorf("error writing heterogeneousFooter: %v", err)
	}

	// write delimiter
	_, err = e.w.Write(delim)
	if err != nil {
		return fmt.Errorf("error writing delimiter: %v", err)
	}
	return nil
}

// HeterogeneousDecoder reads memdumps from the provided reader
type HeterogeneousDecoder struct {
	r           io.Reader
	dr          *DelimitedReader
	hasprotocol bool
}

// NewHeterogeneousDecoder creates a HeterogeneousDecoder that reads memdumps
func NewHeterogeneousDecoder(r io.Reader) *HeterogeneousDecoder {
	return &HeterogeneousDecoder{
		r:  r,
		dr: NewDelimitedReader(r),
	}
}

// Decode reads an object of the specified type from the input.
// The object passed to Decode must be a pointer to the type
// was originally passed to Encode().
func (d *HeterogeneousDecoder) Decode(dest interface{}) error {
	t := reflect.TypeOf(dest)
	if t.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("expected a pointer but got %T", dest))
	}

	ptr, err := d.DecodePtr(t.Elem())
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
func (d *HeterogeneousDecoder) DecodePtr(typ reflect.Type) (interface{}, error) {
	// read protocol
	if !d.hasprotocol {
		var protocol int32
		err := binary.Read(d.r, binary.LittleEndian, &protocol)
		if err != nil {
			return nil, fmt.Errorf("error reading protocol: %v", err)
		}
		if protocol != heterogeneousProtocol {
			return nil, fmt.Errorf("invalid protocol %d", protocol)
		}
		d.hasprotocol = true
	}

	// first segment: read the memory buffer
	dataseg, err := d.dr.Next()
	if len(dataseg) == 0 && err == io.EOF {
		return nil, io.EOF
	}
	if err != nil {
		return nil, fmt.Errorf("error reading data segment: %v", err)
	}

	// read the footer
	footerseg, err := d.dr.Next()
	if err != nil {
		return nil, fmt.Errorf("error reading footer segment: %v", err)
	}

	// decode footer
	var f heterogeneousFooter
	dec := gob.NewDecoder(bytes.NewBuffer(footerseg))
	err = dec.Decode(&f)
	if err != nil {
		return nil, fmt.Errorf("error decoding footer: %v", err)
	}

	// compare descriptors
	descr := describe(typ)
	if !descriptorsEqual(descr, f.Descriptor) {
		return nil, ErrIncompatibleLayout
	}

	// relocate the data
	return relocate(dataseg, f.Pointers, f.Main, typ)
}
