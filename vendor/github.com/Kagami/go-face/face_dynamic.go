//go:build !static
package face

// #cgo LDFLAGS: -ldlib -lblas -lcblas -llapack -ljpeg
import "C"
