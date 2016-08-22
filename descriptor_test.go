package memdump

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func assertCompareDescriptors(t *testing.T, a interface{}, b interface{}, expected bool) {
	da := describe(reflect.TypeOf(a))
	db := describe(reflect.TypeOf(b))
	assert.Equal(t, expected, descriptorsEqual(da, db))
}

func TestDescribeScalar(t *testing.T) {
	assertCompareDescriptors(t, "abc", "d", true)
	assertCompareDescriptors(t, 123, 456, true)
	assertCompareDescriptors(t, 1.2, 3.4, true)
	assertCompareDescriptors(t, uint64(12), uint64(34), true)

	assertCompareDescriptors(t, int16(12), int64(34), false)
	assertCompareDescriptors(t, uint64(12), int64(34), false)
	assertCompareDescriptors(t, uint64(12), "34", false)
	assertCompareDescriptors(t, uint64(12), 4.5, false)
}

func TestDescribePointer(t *testing.T) {
	var x int
	var y uint64
	var z string

	assertCompareDescriptors(t, &x, &x, true)
	assertCompareDescriptors(t, &y, &y, true)
	assertCompareDescriptors(t, &z, &z, true)

	assertCompareDescriptors(t, &x, &y, false)
	assertCompareDescriptors(t, &x, &z, false)
	assertCompareDescriptors(t, &x, x, false)

	xptr := &x
	xptrptr := &xptr

	assertCompareDescriptors(t, xptrptr, xptrptr, true)
	assertCompareDescriptors(t, xptr, xptrptr, false)
	assertCompareDescriptors(t, x, xptrptr, false)
}

func TestDescribeSlice(t *testing.T) {
	var x []int
	var y []uint64
	var z [][]int16

	assertCompareDescriptors(t, x, x, true)
	assertCompareDescriptors(t, y, y, true)
	assertCompareDescriptors(t, z, z, true)
	assertCompareDescriptors(t, &x, &x, true)
	assertCompareDescriptors(t, &y, &y, true)
	assertCompareDescriptors(t, &z, &z, true)

	assertCompareDescriptors(t, x, y, false)
	assertCompareDescriptors(t, &x, &y, false)
	assertCompareDescriptors(t, x, z, false)
	assertCompareDescriptors(t, &x, x, false)
}

func TestDescribeArray(t *testing.T) {
	var x [4]int
	var y [10]int
	var z [2][3]int16

	assertCompareDescriptors(t, x, x, true)
	assertCompareDescriptors(t, y, y, true)
	assertCompareDescriptors(t, z, z, true)
	assertCompareDescriptors(t, &x, &x, true)
	assertCompareDescriptors(t, &y, &y, true)
	assertCompareDescriptors(t, &z, &z, true)

	assertCompareDescriptors(t, x, y, false)
	assertCompareDescriptors(t, &x, &y, false)
	assertCompareDescriptors(t, x, z, false)
	assertCompareDescriptors(t, &x, x, false)
}

func TestDescribeStruct(t *testing.T) {
	type u struct {
		A string
		B int
	}
	type v struct {
		A string
		B int32
	}
	type w struct {
		u
		A string
		B int
		c v
	}

	assertCompareDescriptors(t, u{}, u{}, true)
	assertCompareDescriptors(t, v{}, v{}, true)
	assertCompareDescriptors(t, w{}, w{}, true)
	assertCompareDescriptors(t, &u{}, &u{}, true)
	assertCompareDescriptors(t, &v{}, &v{}, true)
	assertCompareDescriptors(t, &w{}, &w{}, true)

	assertCompareDescriptors(t, u{}, v{}, false)
	assertCompareDescriptors(t, u{}, w{}, false)
	assertCompareDescriptors(t, v{}, w{}, false)
	assertCompareDescriptors(t, u{}, &u{}, false)
}

func TestDescribeStructWithTags(t *testing.T) {
	type u struct {
		A string
		B int
	}
	type v struct {
		A   string
		xyz int
	}
	type w struct {
		A string
		B int `memdump:"xyz"`
	}

	assertCompareDescriptors(t, u{}, v{}, false)
	assertCompareDescriptors(t, u{}, w{}, false)
	assertCompareDescriptors(t, v{}, w{}, true)
}

func TestDescriptorsEqual_DifferentElems(t *testing.T) {
	type u struct {
		A []string
	}
	type v struct {
		A []int
	}
	type w struct {
		A []int
	}

	assertCompareDescriptors(t, u{}, v{}, false)
	assertCompareDescriptors(t, u{}, w{}, false)
	assertCompareDescriptors(t, v{}, w{}, true)
}

func TestDescriptorsEqual_DifferentNumFields(t *testing.T) {
	type u struct {
		A string
		B string
	}
	type v struct {
		A string
	}
	type w struct {
		A string
	}

	assertCompareDescriptors(t, u{}, v{}, false)
	assertCompareDescriptors(t, u{}, w{}, false)
	assertCompareDescriptors(t, v{}, w{}, true)
}

func TestDescribe_PanicsOnMap(t *testing.T) {
	type T struct {
		A map[string]int
	}
	assert.Panics(t, func() {
		describe(reflect.TypeOf(T{}))
	})
}
