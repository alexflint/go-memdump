// Package bench contains benchmarks that compare memdump with other
// serialization packages. This code is in a separate package to avoid
// introducing unnecessary dependencies to memdump.

package bench

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"testing"

	memdump "github.com/alexflint/go-memdump"
)

type pathComponent struct {
	S string
	R int
}

type treeNode struct {
	Label    string
	Weight   int
	Path     []pathComponent
	Children []*treeNode
}

func generateTree(depth, degree int) *treeNode {
	tpl := treeNode{
		Label:  "label",
		Weight: 123,
		Path: []pathComponent{
			{"abc", 4},
			{"def", 5},
			{"ghi", 6},
		},
	}

	root := tpl
	cur := []*treeNode{&root}
	var next []*treeNode
	for i := 1; i < depth; i++ {
		for _, n := range cur {
			for j := 0; j < degree; j++ {
				ch := tpl
				n.Children = append(n.Children, &ch)
				next = append(next, &ch)
			}
		}
		//log.Printf("at i=%d, len=%d", i, len(next))
		cur, next = next, cur[:0]
	}
	return &root
}

func requireNoError(b *testing.B, err error) {
	if err != nil {
		b.Error(err)
		b.FailNow()
	}
}

const minDepth = 16
const maxDepth = 16
const degree = 2

func BenchmarkHomogeneousMemdump(b *testing.B) {
	var bufs [][]byte
	for i := minDepth; i <= maxDepth; i++ {
		in := generateTree(i, degree)
		var buf bytes.Buffer
		enc := memdump.NewEncoder(&buf)
		err := enc.Encode(in)
		requireNoError(b, err)
		bufs = append(bufs, buf.Bytes())
	}

	for d, buf := range bufs {
		b.Run(fmt.Sprintf("depth=%d", d), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				dec := memdump.NewDecoder(bytes.NewBuffer(buf))
				var out treeNode
				err := dec.Decode(&out)
				requireNoError(b, err)
			}
		})
	}
}

func BenchmarkSingleMemdump(b *testing.B) {
	var bufs [][]byte
	for i := minDepth; i <= maxDepth; i++ {
		in := generateTree(i, degree)
		var buf bytes.Buffer
		err := memdump.Encode(&buf, in)
		requireNoError(b, err)
		bufs = append(bufs, buf.Bytes())
	}

	for d, buf := range bufs {
		b.Run(fmt.Sprintf("depth=%d", d), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var out *treeNode
				err := memdump.Decode(bytes.NewBuffer(buf), &out)
				requireNoError(b, err)
			}
		})
	}
}

func BenchmarkGob(b *testing.B) {
	var bufs [][]byte
	for i := minDepth; i <= maxDepth; i++ {
		in := generateTree(i, degree)
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		err := enc.Encode(in)
		requireNoError(b, err)
		bufs = append(bufs, buf.Bytes())
	}

	for d, buf := range bufs {
		b.Run(fmt.Sprintf("depth=%d", d), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				dec := gob.NewDecoder(bytes.NewBuffer(buf))
				var out treeNode
				err := dec.Decode(&out)
				requireNoError(b, err)
			}
		})
	}
}

func BenchmarkJSON(b *testing.B) {
	var bufs [][]byte
	for i := minDepth; i <= maxDepth; i++ {
		in := generateTree(i, degree)
		buf, err := json.Marshal(in)
		requireNoError(b, err)
		bufs = append(bufs, buf)
	}

	for d, buf := range bufs {
		b.Run(fmt.Sprintf("depth=%d", d), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var out treeNode
				err := json.Unmarshal(buf, &out)
				requireNoError(b, err)
			}
		})
	}
}
