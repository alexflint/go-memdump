package memdump

import (
	"encoding/binary"
	"fmt"
	"io"
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

// memEncoderState contains the state that is local to a single Encode() call.
type memEncoderState struct {
	ptrLocs []int64
	next    uintptr
}

// alloc makes room for N objects of the specified type, and returns the
// base offset for that object. It deals correctly with alignment.
func (e *memEncoderState) alloc(t reflect.Type, n int) uintptr {
	align := uintptr(t.Align())
	if e.next%align != 0 {
		e.next += align - (e.next % align)
	}
	cur := e.next
	e.next += t.Size() * uintptr(n)
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
func (e *memEncoder) Encode(ptr interface{}) ([]int64, error) {
	var state memEncoderState

	ptrval := reflect.ValueOf(ptr)
	objval := ptrval.Elem()
	cache := make(map[uintptr]uintptr)
	queue := []block{{
		src:  objval,
		dest: state.alloc(objval.Type(), 1),
	}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		blockaddr := cur.src.Addr()
		blockbytes := asBytes(cur.src)

		fmt.Printf("at %d (%v)\n", e.w.offset, cur.src.Type())

		// check the position of the writer
		if cur.dest < uintptr(e.w.offset) {
			panic(fmt.Sprintf("block.dest=%d but writer is at %d", cur.dest, e.w.offset))
		}

		// for byte-alignment purposes we may need to fill some bytes
		if fill := cur.dest - uintptr(e.w.offset); fill > 0 {
			fmt.Println("filling", fill)
			_, err := e.w.Write(make([]byte, fill))
			if err != nil {
				return nil, err
			}
		}

		// look up info about this type
		info := lookupType(cur.src.Type())

		// add each referenced object to the queue
		var blockpos uintptr
		for _, ptr := range info.pointers {
			_, err := e.w.Write(blockbytes[blockpos:ptr.offset])
			if err != nil {
				return nil, err
			}

			ptrdata := unsafe.Pointer(blockaddr.Pointer() + ptr.offset)
			ptrval := reflect.NewAt(ptr.typ, ptrdata).Elem()

			var dest uintptr
			if !isNil(ptrval) {
				state.ptrLocs = append(state.ptrLocs, int64(cur.dest+ptr.offset))

				var found bool
				dest, found = cache[readPointer(ptrval)]
				if !found {
					switch ptr.typ.Kind() {
					case reflect.Ptr:
						dest = state.alloc(ptr.typ.Elem(), 1)
						queue = append(queue, block{
							src:  ptrval.Elem(),
							dest: dest,
						})
					case reflect.Slice:
						dest = state.alloc(ptr.typ.Elem(), ptrval.Len())
						arr := arrayFromSlice(ptrval)
						queue = append(queue, block{
							src:  arr,
							dest: dest,
						})
					case reflect.String:
						dest = state.alloc(byteType, ptrval.Len())
						arr := arrayFromString(ptrval)
						queue = append(queue, block{
							src:  arr,
							dest: dest,
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

		_, err := e.w.Write(blockbytes[blockpos:])
		if err != nil {
			return nil, err
		}
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
