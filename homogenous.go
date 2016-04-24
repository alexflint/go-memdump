package memdump

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"reflect"
)

// header is gob-encoded in the first segment
type header struct {
	Protocol   int32
	Descriptor descriptor
}

// footer is binary-encoded after each data segment
type footer struct {
	Main     int   // Main contains the offset of the primary object
	Pointers []int // Pointers contains the offset of each pointer
}

func encodeFooter(w io.Writer, f *footer) error {
	err := binary.Write(w, binary.LittleEndian, int64(len(f.Pointers)))
	if err != nil {
		return err
	}

	err = binary.Write(w, binary.LittleEndian, int64(f.Main))
	if err != nil {
		return err
	}

	pointers := make([]int64, len(f.Pointers))
	for i, ptr := range f.Pointers {
		pointers[i] = int64(ptr)
	}

	err = binary.Write(w, binary.LittleEndian, pointers)
	if err != nil {
		return err
	}

	return nil
}

func decodeFooter(r io.Reader, f *footer) error {
	// read the number of pointers
	var n int64
	err := binary.Read(r, binary.LittleEndian, &n)
	if err != nil {
		return err
	}

	// read struct fields
	var main int64
	err = binary.Read(r, binary.LittleEndian, &main)
	if err != nil {
		return err
	}

	pointers := make([]int64, n)
	err = binary.Read(r, binary.LittleEndian, pointers)
	if err != nil {
		return err
	}

	f.Main = int(main)
	f.Pointers = make([]int, n)
	for i, ptr := range pointers {
		f.Pointers[i] = int(ptr)
	}

	return nil
}

// Encoder writes memdumps to the provided writer
type Encoder struct {
	w io.Writer
	t reflect.Type
}

// NewEncoder creates an Encoder that writes memdumps to the provided writer
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w: w,
	}
}

// Encode writes a memdump of the provided object to output. You must pass a
// pointer to the object you wish to encode. To encode a pointer, pass a
// double-pointer.
func (e *Encoder) Encode(obj interface{}) error {
	t := reflect.TypeOf(obj)
	if t.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("expected a pointer but got %T", obj))
	}
	if e.t != nil && e.t != t {
		panic(fmt.Sprintf("each call to Encode should pass the same type, but got %v then %v", e.t, t))
	}

	if e.t == nil {
		// write the header
		gob := gob.NewEncoder(e.w)
		err := gob.Encode(header{
			Protocol:   homogeneousProtocol,
			Descriptor: describe(t.Elem()),
		})
		if err != nil {
			return fmt.Errorf("error writing footer: %v", err)
		}

		e.t = t
		_, err = e.w.Write(delim)
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

	// second segment: write the footer
	err = encodeFooter(e.w, &footer{Pointers: ptrs})
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
	dr *DelimitedReader
	t  reflect.Type
}

// New creates a Decoder that reads memdumps
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		dr: NewDelimitedReader(r),
	}
}

// Decode reads an object of the specified type from the input.
// The object passed to Decode must be a pointer to the type
// was originally passed to Encode().
func (d *Decoder) Decode(dest interface{}) error {
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
func (d *Decoder) DecodePtr(t reflect.Type) (interface{}, error) {
	if d.t != nil && d.t != t {
		panic(fmt.Sprintf("each call to Encode should pass the same type, but got %v then %v", d.t, t))
	}

	// read the header
	if d.t == nil {
		// decode the descriptor
		seg, err := d.dr.Next()
		if err != nil {
			return nil, fmt.Errorf("error reading header segment: %v", err)
		}

		var header header
		dec := gob.NewDecoder(bytes.NewBuffer(seg))
		err = dec.Decode(&header)
		if err != nil {
			return nil, fmt.Errorf("error decoding header: %v", err)
		}

		// compare descriptors
		expectedDescr := describe(t)
		if !descriptorsEqual(expectedDescr, header.Descriptor) {
			return nil, ErrIncompatibleLayout
		}

		d.t = t
	}

	// read the data
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
		return nil, fmt.Errorf("error decoding footer: %v", err)
	}

	// decode footer
	var f footer
	err = decodeFooter(bytes.NewBuffer(footerseg), &f)
	if err != nil {
		return nil, fmt.Errorf("error decoding footer: %v", err)
	}

	// relocate the data
	return relocate(dataseg, f.Pointers, f.Main, t)
}
