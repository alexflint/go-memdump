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

// Decode reads an object of the specified type from the input
// and stores a pointer to it at the location specified by ptrptr,
// which must be a pointer to a pointer. If you originally called
// Encode with parameter *T then you should pass **T to Decode.
func Decode(r io.Reader, ptrptr interface{}) error {
	v := reflect.ValueOf(ptrptr)
	t := v.Type()
	if t.Kind() != reflect.Ptr || t.Elem().Kind() != reflect.Ptr {
		panic(fmt.Sprintf("expected a pointer to a pointer but got %v", v.Type()))
	}

	// read the locations
	var loc locations
	err := decodeLocations(r, &loc)
	if err != nil {
		return fmt.Errorf("error decoding relocation data: %v", err)
	}

	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return fmt.Errorf("error reading data segment: %v", err)
	}

	// relocate the data
	out, err := relocate(buf, loc.Pointers, loc.Main, v.Type().Elem().Elem())
	if err != nil {
		return fmt.Errorf("error relocating data: %v", err)
	}

	v.Elem().Set(reflect.ValueOf(out))
	return nil
}
