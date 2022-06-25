[![Documentation](https://godoc.org/github.com/alexflint/go-memdump?status.svg)](https://godoc.org/github.com/alexflint/go-memdump)
[![Build Status](https://github.com/alexflint/go-memdump/workflows/Go/badge.svg)](https://github.com/alexflint/go-memdump/actions)
[![Coverage Status](https://coveralls.io/repos/github/alexflint/go-memdump/badge.svg?branch=master)](https://coveralls.io/github/alexflint/go-memdump?branch=master)
[![Report Card](https://goreportcard.com/badge/github.com/alexflint/go-memdump)](https://goreportcard.com/report/github.com/alexflint/go-memdump)

## Very fast, very unsafe serialization for Go

This package provides a fast way to load large amounts of data into Go structs. Memdump can load datasets containing millions of small structs at over 1 GB/s (compared to ~30 MB/s for gob or json).

The price you pay is:
- you cannot load structs that contain maps or interfaces
- your data is not portable across machine architectures (64 bit vs 32 bit, big-endian vs small-endian)

### Benchmarks

The benchmarks were measured by encoding and decoding a tree containing 2,097,151 small structs. Code is under the bench dir.

**Decode**
```
                 gob    28.17 MB/s      (39.8 MB in 1.41s)
                json    30.17 MB/s      (113.8 MB in 3.77s)
             memdump  1031.54 MB/s      (113.2 MB in 0.11s)
```

**Encode**
```
                 gob    37.07 MB/s      (39.8 MB in 1.07s)
                json    77.20 MB/s      (113.8 MB in 1.47s)
             memdump    61.25 MB/s      (113.2 MB in 1.85s)
```

To reproduce these results:
```shell
$ go run ./bench/summarize.go
```

## Quick start

```shell
go get github.com/alexflint/go-memdump
```

Write data to a file:

```go
type data struct {
	Foo string
	Bar int
}

w, err := os.Create("/tmp/data.memdump")
if err != nil {
	...
}

// note that you must pass a pointer when encoding
mydata := data{Foo: "abc", Bar: 123}
memdump.Encode(w, &mydata)
```

Load data from a file:

```go
r, err := os.Open("/tmp/data.memdump")
if err != nil {
	...
}

// note that you muss pass a pointer to a pointer when decoding
var mydata *data
memdump.Decode(r, &mydata)
```
