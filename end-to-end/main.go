package main

import (
	"bytes"
	"fmt"
	"os"
	"reflect"

	memdump "github.com/alexflint/go-memdump"
)

type T struct {
	X int
	Y string
}

func main() {
	var b bytes.Buffer

	in := T{X: 123, Y: "xyz"}
	err := memdump.Encode(&b, &in)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var out *T
	err = memdump.Decode(&b, &out)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if !reflect.DeepEqual(in, *out) {
		fmt.Printf("objects were not equal: %v vs %v\n", in, *out)
		os.Exit(1)
	}
}
