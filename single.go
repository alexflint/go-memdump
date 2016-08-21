package memdump

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
)

// Encode writes a memdump of the provided object to output. You must
// pass a pointer to the object you wish to encode.
func Encode(w io.Writer, obj interface{}) error {
	t := reflect.TypeOf(obj)
	if t.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("expected a pointer but got %T", obj))
	}

	// write the object data to a temporary buffer
	var buf bytes.Buffer
	mem := newMemEncoder(&buf)
	ptrs, err := mem.Encode(obj)
	if err != nil {
		return fmt.Errorf("error while walking data: %v", err)
	}

	// write the locations at the top
	err = encodeLocations(w, &locations{Pointers: ptrs})
	if err != nil {
		return fmt.Errorf("error writing location segment: %v", err)
	}

	// now write the data segment
	_, err = buf.WriteTo(w)
	if err != nil {
		return fmt.Errorf("error writing data segment: %v", err)
	}

	return nil
}

// Decode reads an object of the specified type from the input.
// The object passed to Decode must be a pointer to the type
// was originally passed to Encode().
func Decode(r io.Reader, dest interface{}) error {
	t := reflect.TypeOf(dest)
	if t.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("expected a pointer but got %T", dest))
	}

	ptr, err := DecodePtr(r, t.Elem())
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
func DecodePtr(r io.Reader, t reflect.Type) (interface{}, error) {
	// read the locations
	var loc locations
	err := decodeLocations(r, &loc)
	if err != nil {
		return nil, fmt.Errorf("error decoding relocation data: %v", err)
	}

	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("error reading data segment: %v", err)
	}

	// relocate the data
	return relocate(buf, loc.Pointers, loc.Main, t)
}
