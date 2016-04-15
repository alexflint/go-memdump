package memdump

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"reflect"
	"sort"
	"sync"
	"unsafe"
)

// uintptrSize is the size in bytes of uintptr
const uintptrSize = unsafe.Sizeof(uintptr(0))

// byteType is the reflect.Type of byte
var (
	byteType      = reflect.TypeOf(byte(0))
	typeCache     = make(map[reflect.Type]*typeInfo)
	typeCacheLock sync.Mutex
)

// block represents a value to be written to the stream
type block struct {
	src  reflect.Value
	dest uintptr
	size uintptr
}

// pointer represents the location of a pointer in a type
type pointer struct {
	offset uintptr
	typ    reflect.Type
}

// typeInfo represents the location of the pointers in a type
type typeInfo struct {
	pointers []pointer
}

// asBytes gets a byte slice with data pointer set to the address of the
// assigned value and length set to the sizeof the value.
func asBytes(v reflect.Value) []byte {
	size := int(v.Type().Size())
	hdr := reflect.SliceHeader{
		Data: v.Addr().Pointer(),
		Len:  size,
		Cap:  size,
	}
	return *(*[]byte)(unsafe.Pointer(&hdr))
}

// isNil determines whether the pointer contained within v is nil.
// This is equivalent to checking x==nil, except for strings, where
// this method checks the data pointer inside the string header.
func isNil(v reflect.Value) bool {
	if v.Kind() == reflect.String {
		hdr := (*reflect.StringHeader)(unsafe.Pointer(v.Addr().Pointer()))
		return hdr.Data == 0
	}
	return v.IsNil()
}

func readPointer(v reflect.Value) uintptr {
	if v.Kind() == reflect.String {
		hdr := (*reflect.StringHeader)(unsafe.Pointer(v.Addr().Pointer()))
		return hdr.Data
	}
	return v.Pointer()
}

type byOffset []pointer

func (xs byOffset) Len() int           { return len(xs) }
func (xs byOffset) Swap(i, j int)      { xs[i], xs[j] = xs[j], xs[i] }
func (xs byOffset) Less(i, j int) bool { return xs[i].offset < xs[j].offset }

type countingWriter struct {
	w      io.Writer
	offset int
}

func (w *countingWriter) Write(buf []byte) (int, error) {
	n, err := w.w.Write(buf)
	w.offset += n
	return n, err
}

// memEncoder writes the in-memory representation of an object, together
// with all referenced objects.
type memEncoder struct {
	w countingWriter
}

func newMemEncoder(w io.Writer) *memEncoder {
	return &memEncoder{
		w: countingWriter{w: w},
	}
}

// memEncoderState contains the state that is only kept within a single Encode() call.
type memEncoderState struct {
	ptrLocs []int
	next    uintptr
}

// alloc makes room for an object of the specified size, and returns the
// base offset for that object.
func (e *memEncoderState) alloc(size uintptr) uintptr {
	cur := e.next
	e.next += size
	return cur
}

// arrayFromString gets a fixed-size array representing the bytes pointed to
// by a string.
func arrayFromString(strval reflect.Value) reflect.Value {
	if strval.Kind() != reflect.String {
		panic(fmt.Sprintf("expected string type but got %s", strval.Type()))
	}

	hdr := (*reflect.StringHeader)(unsafe.Pointer(strval.Addr().Pointer()))
	typ := reflect.ArrayOf(hdr.Len, byteType)
	return reflect.NewAt(typ, unsafe.Pointer(hdr.Data)).Elem()
}

// arrayFromSlice gets a fixed-size array representing the data pointed to by
// a slice
func arrayFromSlice(sliceval reflect.Value) reflect.Value {
	if sliceval.Kind() != reflect.Slice {
		panic(fmt.Sprintf("expected string type but got %s", sliceval.Type()))
	}

	hdr := (*reflect.SliceHeader)(unsafe.Pointer(sliceval.Addr().Pointer()))
	typ := reflect.ArrayOf(hdr.Len, sliceval.Type().Elem())
	return reflect.NewAt(typ, unsafe.Pointer(hdr.Data)).Elem()
}

// Encode writes the in-memory representation of the object pointed to by ptr. It
// returns the offset of each pointer and an error.
func (e *memEncoder) Encode(ptr interface{}) ([]int, error) {
	var state memEncoderState

	ptrval := reflect.ValueOf(ptr)
	objval := ptrval.Elem()

	cache := make(map[uintptr]uintptr)

	queue := []block{block{
		src:  objval,
		dest: state.alloc(objval.Type().Size()),
		size: objval.Type().Size(),
	}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		blockaddr := cur.src.Addr().Pointer()
		blockbytes := asBytes(cur.src)
		log.Printf("at %v (%d bytes)", cur.src.Type(), len(blockbytes))
		if len(blockbytes) != int(cur.size) {
			panic(fmt.Sprintf("expected %v to be %d bytes but turned out to be %d bytes",
				cur.src.Type(), cur.size, len(blockbytes)))
		}

		if cur.dest != uintptr(e.w.offset) {
			panic(fmt.Sprintf("block.dest=%d but writer is at %d", cur.dest, e.w.offset))
		}

		info := lookupType(cur.src.Type())
		log.Printf("  found %d pointers", len(info.pointers))

		var blockpos uintptr
		for _, ptr := range info.pointers {
			_, err := e.w.Write(blockbytes[blockpos:ptr.offset])
			if err != nil {
				return nil, err
			}

			log.Printf("  translating %s at %d", ptr.typ, int(cur.dest+ptr.offset))

			ptraddr := blockaddr + ptr.offset
			ptrdata := unsafe.Pointer(ptraddr)
			ptrval := reflect.NewAt(ptr.typ, ptrdata).Elem()

			var dest uintptr
			if isNil(ptrval) {
				log.Println("    pointer is nil")
			} else {
				state.ptrLocs = append(state.ptrLocs, int(cur.dest+ptr.offset))

				var found bool
				dest, found = cache[readPointer(ptrval)]
				if !found {
					switch ptr.typ.Kind() {
					case reflect.Ptr:
						size := ptr.typ.Elem().Size()
						dest = state.alloc(size)
						queue = append(queue, block{
							src:  ptrval.Elem(),
							dest: dest,
							size: size,
						})
					case reflect.Slice:
						size := uintptr(ptrval.Len()) * ptr.typ.Elem().Size()
						dest = state.alloc(size)
						arr := arrayFromSlice(ptrval)
						queue = append(queue, block{
							src:  arr,
							dest: dest,
							size: size,
						})
					case reflect.String:
						size := uintptr(ptrval.Len())
						dest = state.alloc(size)
						arr := arrayFromString(ptrval)
						queue = append(queue, block{
							src:  arr,
							dest: dest,
							size: size,
						})
					}
					cache[readPointer(ptrval)] = dest
				}
			}

			err = binary.Write(&e.w, binary.LittleEndian, uint64(dest))
			if err != nil {
				return nil, err
			}
			blockpos = ptr.offset + uintptrSize
		}
		e.w.Write(blockbytes[blockpos:])
	}

	return state.ptrLocs, nil
}

// pointerFinder gets the byte offset of each pointer in an object. It
// only considers the immediate value of an object (i.e. the bytes that
// would be copied in a simple assignment). It does not follow pointers
// to other objects.
type pointerFinder struct {
	pointers []pointer
}

func (f *pointerFinder) visit(t reflect.Type, base uintptr) {
	switch t.Kind() {
	case reflect.Ptr, reflect.String, reflect.Slice:
		// these four types all store one pointer at offset zero
		f.pointers = append(f.pointers, pointer{
			offset: base,
			typ:    t,
		})
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			f.visit(field.Type, base+field.Offset)
		}
	case reflect.Array:
		elemSize := t.Elem().Size()
		elemPtrs := lookupType(t.Elem()).pointers
		for _, elemPtr := range elemPtrs {
			for i := 0; i < t.Len(); i++ {
				f.pointers = append(f.pointers, pointer{
					offset: base + uintptr(i)*elemSize + elemPtr.offset,
					typ:    elemPtr.typ,
				})
			}
		}
	case reflect.Map, reflect.Chan, reflect.Interface, reflect.UnsafePointer, reflect.Func:
		panic(fmt.Sprintf("cannot serialize objects of %v kind (got %v)", t.Kind(), t))
	}
}

// lookupType gets the type info for t.
func lookupType(t reflect.Type) *typeInfo {
	typeCacheLock.Lock()
	info, found := typeCache[t]
	typeCacheLock.Unlock()

	if !found {
		var f pointerFinder
		f.visit(t, 0)
		info = &typeInfo{pointers: f.pointers}
		sort.Sort(byOffset(info.pointers))

		typeCacheLock.Lock()
		typeCache[t] = info
		typeCacheLock.Unlock()
	}

	return info
}

// relocate adds the base address to each pointer in the buffer, then reinterprets
// the buffer as an object of type t.
func relocate(buf []byte, f *footer, t reflect.Type) (interface{}, error) {
	base := uintptr(unsafe.Pointer(&buf[0]))
	for i, loc := range f.Pointers {
		if loc < 0 || loc >= len(buf) {
			return nil, fmt.Errorf("pointer %d was out of range: %d (buffer len=%d)", i, loc, len(buf))
		}
		v := (*uintptr)(unsafe.Pointer(&buf[loc]))
		*v += base
	}
	if f.Main < 0 || f.Main >= len(buf) {
		return nil, fmt.Errorf("footer.Main was out of range: %d (buffer len=%d)", f.Main, len(buf))
	}
	return reflect.NewAt(t, unsafe.Pointer(&buf[f.Main])).Interface(), nil
}
