package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"os"
	"time"

	arg "github.com/alexflint/go-arg"
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

type codec interface {
	name() string
	encode(t *treeNode) ([]byte, error)
	decode([]byte) error
}

type gobcodec struct{}

func (gobcodec) name() string { return "gob" }

func (gobcodec) encode(t *treeNode) ([]byte, error) {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	err := enc.Encode(t)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (gobcodec) decode(buf []byte) error {
	dec := gob.NewDecoder(bytes.NewBuffer(buf))
	var t treeNode
	return dec.Decode(&t)
}

type jsoncodec struct{}

func (jsoncodec) name() string { return "json" }

func (jsoncodec) encode(t *treeNode) ([]byte, error) {
	return json.Marshal(t)
}

func (jsoncodec) decode(buf []byte) error {
	var t treeNode
	return json.Unmarshal(buf, &t)
}

type memdumpcodec struct{}

func (memdumpcodec) name() string { return "memdump" }

func (memdumpcodec) encode(t *treeNode) ([]byte, error) {
	var b bytes.Buffer
	err := memdump.Encode(&b, t)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (memdumpcodec) decode(buf []byte) error {
	var t *treeNode
	return memdump.Decode(bytes.NewBuffer(buf), &t)
}

func main() {
	var args struct {
		Repeat int
		Depth  int
		Degree int
	}
	args.Repeat = 5
	args.Depth = 20
	args.Degree = 2
	arg.MustParse(&args)

	t := generateTree(args.Depth, args.Degree)

	fmt.Println("DECODE")
	for _, codec := range []codec{gobcodec{}, jsoncodec{}, memdumpcodec{}} {
		buf, err := codec.encode(t)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		begin := time.Now()
		for i := 0; i < args.Repeat; i++ {
			err := codec.decode(buf)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
		duration := time.Since(begin).Seconds() / float64(args.Repeat)

		bytesPerSec := float64(len(buf)) / duration

		fmt.Printf("%20s %8.2f MB/s      (%.1f MB in %.2fs)\n",
			codec.name(), bytesPerSec/1000000., float64(len(buf))/1000000., duration)
	}

	fmt.Println("ENCODE")
	for _, codec := range []codec{gobcodec{}, jsoncodec{}, memdumpcodec{}} {
		buf, err := codec.encode(t)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		begin := time.Now()
		for i := 0; i < args.Repeat; i++ {
			codec.encode(t)
		}
		duration := time.Since(begin).Seconds() / float64(args.Repeat)

		bytesPerSec := float64(len(buf)) / duration

		fmt.Printf("%20s %8.2f MB/s      (%.1f MB in %.2fs)\n",
			codec.name(), bytesPerSec/1000000., float64(len(buf))/1000000., duration)
	}
}
