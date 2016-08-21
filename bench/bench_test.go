// Package bench contains benchmarks that compare memdump with other
// serialization packages. This code is in a separate package to avoid
// introducing unnecessary dependencies to memdump.

package bench

import (
	"bytes"
	"reflect"
	"testing"

	memdump "github.com/alexflint/go-memdump"
	humanize "github.com/dustin/go-humanize"
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

const minDepth = 8
const maxDepth = 16
const degree = 2

func BenchmarkHomogeneousMemdump(b *testing.B) {
	var bufs [][]byte
	for i := minDepth; i <= maxDepth; i++ {
		in := generateTree(i, degree)
		var buf bytes.Buffer
		enc := memdump.NewEncoder(&buf)
		enc.Encode(in)
		bufs = append(bufs, buf.Bytes())
	}

	for _, buf := range bufs {
		label := humanize.Bytes(uint64(len(buf)))
		b.Run(label, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				dec := memdump.NewDecoder(bytes.NewBuffer(buf))
				var out treeNode
				err := dec.Decode(&out)
				if err != nil {
					b.Error(err)
					b.FailNow()
				}
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
		if err != nil {
			b.Error(err)
			b.FailNow()
		}
		bufs = append(bufs, buf.Bytes())
	}

	for _, buf := range bufs {
		label := humanize.Bytes(uint64(len(buf)))
		b.Run(label, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				//var out treeNode
				//err := memdump.Decode(bytes.NewBuffer(buf), &out)
				_, err := memdump.DecodePtr(bytes.NewBuffer(buf), reflect.TypeOf(treeNode{}))
				if err != nil {
					b.Error(err)
					b.FailNow()
				}
			}
		})
	}
}
