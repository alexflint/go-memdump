package memdump

import "errors"

// Protocols numbers used: (do not re-use)
//  1: homogeneous protocol, April 20, 2016
//  2: heterogeneous protocol, April 20, 2016

const (
	homogeneousProtocol   int32 = 1
	heterogeneousProtocol int32 = 2
)

var (
	// ErrIncompatibleLayout is returned by decoders when the object on the wire has
	// an in-memory layout that is not compatible with the requested Go type.
	ErrIncompatibleLayout = errors.New("attempted to load data with incompatible layout")
)
