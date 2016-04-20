package memdump

import (
	"fmt"
	"reflect"
)

// pointer - type
// slice - type
// array - type, length
// string
// int - size
// float - size

// A Descriptor describes a type such that if two types have the
// same descriptor then their memory layout is identical.
type descriptor []typ

// a class contains information about a type
type typ struct {
	Kind   reflect.Kind // Kind is the kind of this type
	Size   uintptr      // Size is the size in bits, as per reflect.Value.Size
	Elem   int          // Elem is index of the underlying type for pointers, slices, and arrays
	Fields []field      // Fields contains the fields for structs
}

type field struct {
	Name   string  // Name is the name of this field, or its memdump tag if present
	Offset uintptr // Offset is the position of this field relative to the beginning of the struct
	Type   int     // ID is the index of the type of this field in the descripto
}

// descriptorsEqual compares two descriptors
func descriptorsEqual(a, b descriptor) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Kind != b[i].Kind {
			return false
		}
		if a[i].Size != b[i].Size {
			return false
		}
		if a[i].Elem != b[i].Elem {
			return false
		}
		if len(a[i].Fields) != len(b[i].Fields) {
			return false
		}
		for j := range a[i].Fields {
			if a[i].Fields[j].Name != b[i].Fields[j].Name {
				return false
			}
			if a[i].Fields[j].Offset != b[i].Fields[j].Offset {
				return false
			}
			if a[i].Fields[j].Type != b[i].Fields[j].Type {
				return false
			}
		}
	}
	return true
}

// describe computes the descriptor for a type
func describe(t reflect.Type) descriptor {
	var nextID int
	var desc descriptor
	var queue []reflect.Type
	seen := make(map[reflect.Type]int)

	push := func(t reflect.Type) int {
		if id, found := seen[t]; found {
			return id
		}
		id := nextID
		seen[t] = id
		nextID++
		queue = append(queue, t)
		return id
	}

	push(t)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		t := typ{
			Size: cur.Size(),
			Kind: cur.Kind(),
		}

		switch cur.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map:
			panic(fmt.Sprintf("cannot compute descriptor for %v", cur.Kind()))
		case reflect.Array, reflect.Slice, reflect.Ptr:
			t.Elem = push(cur.Elem())
		case reflect.Struct:
			for i := 0; i < cur.NumField(); i++ {
				f := cur.Field(i)
				if f.Type.Size() == 0 {
					continue
				}

				name := f.Name
				if tag := f.Tag.Get("memdump"); tag != "" {
					name = tag
				}
				t.Fields = append(t.Fields, field{
					Name:   name,
					Offset: f.Offset,
					Type:   push(f.Type),
				})
			}
		}

		desc = append(desc, t)
	}
	return desc
}
