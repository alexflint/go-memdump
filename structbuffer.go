package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"reflect"
	"sort"
	"unsafe"

	"github.com/kr/pretty"
)

func asBytes(v reflect.Value) []byte {
	size := int(v.Type().Size())
	hdr := reflect.SliceHeader{
		Data: v.Addr().Pointer(),
		Len:  size,
		Cap:  size,
	}
	return *(*[]byte)(unsafe.Pointer(&hdr))
}

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

type block struct {
	src  reflect.Value
	dest uintptr
	size uintptr
}

type pointer struct {
	offset uintptr
	typ    reflect.Type
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
	log.Printf("  writing %d bytes at offset %d", len(buf), w.offset)
	n, err := w.w.Write(buf)
	w.offset += n
	return n, err
}

type Encoder struct {
	w       countingWriter
	ptrLocs []int
	next    uintptr
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w: countingWriter{w: w},
	}
}

const uintptrSize = unsafe.Sizeof(uintptr(0))

var byteType = reflect.TypeOf(byte(0))

func (e *Encoder) alloc(size uintptr) uintptr {
	cur := e.next
	e.next += size
	return cur
}

func arrayFromString(strval reflect.Value) reflect.Value {
	if strval.Kind() != reflect.String {
		panic(fmt.Sprintf("expected string type but got %s", strval.Type()))
	}

	hdr := (*reflect.StringHeader)(unsafe.Pointer(strval.Addr().Pointer()))
	typ := reflect.ArrayOf(hdr.Len, byteType)
	return reflect.NewAt(typ, unsafe.Pointer(hdr.Data)).Elem()
}

func arrayFromSlice(sliceval reflect.Value) reflect.Value {
	if sliceval.Kind() != reflect.Slice {
		panic(fmt.Sprintf("expected string type but got %s", sliceval.Type()))
	}

	hdr := (*reflect.SliceHeader)(unsafe.Pointer(sliceval.Addr().Pointer()))
	typ := reflect.ArrayOf(hdr.Len, sliceval.Type().Elem())
	return reflect.NewAt(typ, unsafe.Pointer(hdr.Data)).Elem()
}

func (e *Encoder) Encode(ptr interface{}) error {
	ptrval := reflect.ValueOf(ptr)
	objval := ptrval.Elem()

	cache := make(map[uintptr]uintptr)

	queue := []block{block{
		src:  objval,
		dest: e.alloc(objval.Type().Size()),
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

		pointers := findPointersInType(cur.src.Type())
		log.Printf("found %d pointers", len(pointers))

		sort.Sort(byOffset(pointers))

		var blockpos uintptr
		for _, ptr := range pointers {
			_, err := e.w.Write(blockbytes[blockpos:ptr.offset])
			if err != nil {
				return err
			}

			log.Printf("  translating %s at %d", ptr.typ, int(cur.dest+ptr.offset))

			ptraddr := blockaddr + ptr.offset
			ptrdata := unsafe.Pointer(ptraddr)
			ptrval := reflect.NewAt(ptr.typ, ptrdata).Elem()

			var dest uintptr
			if isNil(ptrval) {
				log.Println("    pointer is nil")
			} else {
				e.ptrLocs = append(e.ptrLocs, int(cur.dest+ptr.offset))

				var found bool
				dest, found = cache[readPointer(ptrval)]
				if !found {
					switch ptr.typ.Kind() {
					case reflect.Ptr:
						size := ptr.typ.Elem().Size()
						dest = e.alloc(size)
						queue = append(queue, block{
							src:  ptrval.Elem(),
							dest: dest,
							size: size,
						})
					case reflect.Slice:
						size := uintptr(ptrval.Len()) * ptr.typ.Elem().Size()
						dest = e.alloc(size)
						arr := arrayFromSlice(ptrval)
						queue = append(queue, block{
							src:  arr,
							dest: dest,
							size: size,
						})
					case reflect.String:
						size := uintptr(ptrval.Len())
						dest = e.alloc(size)
						arr := arrayFromString(ptrval)
						log.Printf("   created %s with addr=%d, data[0]=%v", arr.Type(), arr.Addr().Pointer(), arr.Index(0))
						queue = append(queue, block{
							src:  arr,
							dest: dest,
							size: size,
						})
					}
					cache[readPointer(ptrval)] = dest
				}
			}

			log.Printf("     value=%d", dest)

			err = binary.Write(&e.w, binary.LittleEndian, uint64(dest))
			if err != nil {
				return err
			}
			blockpos = ptr.offset + uintptrSize
		}
		e.w.Write(blockbytes[blockpos:])
	}

	return nil
}

type pointerTypeFinder struct {
	pointers []pointer
}

func (f *pointerTypeFinder) visit(t reflect.Type, base uintptr) {
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
		elemPtrs := findPointersInType(t.Elem())
		for _, elemPtr := range elemPtrs {
			for i := 0; i < t.Len(); i++ {
				f.pointers = append(f.pointers, pointer{
					offset: base + uintptr(i)*elemSize + elemPtr.offset,
					typ:    elemPtr.typ,
				})
			}
		}
	case reflect.Map, reflect.Chan, reflect.Interface, reflect.UnsafePointer, reflect.Func:
		panic(fmt.Sprintf("cannot serialize a %s (got %v)", t.Kind().String(), t))
	}
}

func findPointersInType(t reflect.Type) []pointer {
	var f pointerTypeFinder
	f.visit(t, 0)
	return f.pointers
}

func Relocate(buf []byte, ptrLocs []int, t reflect.Type) interface{} {
	base := uintptr(unsafe.Pointer(&buf[0]))
	log.Printf("base=%d", base)
	for _, loc := range ptrLocs {
		v := (*uintptr)(unsafe.Pointer(&buf[loc]))
		orig := *v
		*v += base // subtract 1 because 0 means nil, and offset 0 is represented by 1
		log.Printf("relocating position %d: %d -> %d", loc, orig, *v)
	}
	return reflect.NewAt(t, unsafe.Pointer(&buf[0])).Interface()
}

func main() {
	// p := Person{
	// 	Name: "alex",
	// 	Age:  123,
	// }

	type U struct {
		X bool
		Y *U
	}

	type T struct {
		A int
		B *int
		C string
		D *int
		E []int
		F *U
	}

	x := 32

	u := U{false, nil}
	u.Y = &u

	obj := T{
		A: 123,
		B: &x,
		C: "abcabcabc",
		D: &x,
		E: []int{9, 8, 7},
		F: &U{X: true, Y: &u},
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	err := enc.Encode(&obj)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Wrote %d bytes:\n", buf.Len())
	log.Println(buf.Bytes())

	log.Printf("There are %d pointers at:\n", len(enc.ptrLocs))
	for _, loc := range enc.ptrLocs {
		log.Println(loc)
	}

	newobj := Relocate(buf.Bytes(), enc.ptrLocs, reflect.TypeOf(T{})).(*T)
	log.Println(reflect.ValueOf(newobj).Type())
	log.Println(reflect.ValueOf(newobj).Pointer())
	pretty.Println(newobj)
}
