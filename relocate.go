package memdump

import (
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"unsafe"
)

// locations contains the locations of pointers in a data segments
type locations struct {
	Main     int   // Main contains the offset of the primary object
	Pointers []int // Pointers contains the offset of each pointer
}

func encodeLocations(w io.Writer, f *locations) error {
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

func decodeLocations(r io.Reader, f *locations) error {
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

// relocate adds the base address to each pointer in the buffer, then reinterprets
// the buffer as an object of type t.
func relocate(buf []byte, ptrs []int, main int, t reflect.Type) (interface{}, error) {
	base := uintptr(unsafe.Pointer(&buf[0]))
	for i, loc := range ptrs {
		if loc < 0 || loc >= len(buf) {
			return nil, fmt.Errorf("pointer %d was out of range: %d (buffer len=%d)", i, loc, len(buf))
		}
		v := (*uintptr)(unsafe.Pointer(&buf[loc]))
		*v += base
	}
	if main < 0 || main >= len(buf) {
		return nil, fmt.Errorf("footer.Main was out of range: %d (buffer len=%d)", main, len(buf))
	}
	return reflect.NewAt(t, unsafe.Pointer(&buf[main])).Interface(), nil
}
