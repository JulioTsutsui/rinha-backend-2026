package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const indexMagic = "RINHA1\x00\x00"

type Index struct {
	data      []byte
	N         int
	K         int
	NBlocks   int
	centroids []float32
	offsets   []uint32
	counts    []uint32
	bboxMin   []int16
	bboxMax   []int16
	blocks    []int16
	labels    []uint8
}

func align64(x int) int { return (x + 63) &^ 63 }

func viewF32(d []byte, off, n int) []float32 {
	return unsafe.Slice((*float32)(unsafe.Pointer(&d[off])), n)
}
func viewU32(d []byte, off, n int) []uint32 {
	return unsafe.Slice((*uint32)(unsafe.Pointer(&d[off])), n)
}
func viewI16(d []byte, off, n int) []int16 {
	return unsafe.Slice((*int16)(unsafe.Pointer(&d[off])), n)
}

func openIndex(path string) (*Index, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := int(st.Size())
	data, err := syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, err
	}
	if size < 64 || string(data[:8]) != indexMagic {
		return nil, fmt.Errorf("bad index magic in %s", path)
	}
	n := int(binary.LittleEndian.Uint32(data[8:12]))
	k := int(binary.LittleEndian.Uint32(data[12:16]))
	nb := int(binary.LittleEndian.Uint32(data[16:20]))

	ix := &Index{data: data, N: n, K: k, NBlocks: nb}
	off := 64
	ix.centroids = viewF32(data, off, k*Dims)
	off = align64(off + k*Dims*4)
	ix.offsets = viewU32(data, off, k+1)
	off = align64(off + (k+1)*4)
	ix.counts = viewU32(data, off, k)
	off = align64(off + k*4)
	ix.bboxMin = viewI16(data, off, k*Dims)
	off = align64(off + k*Dims*2)
	ix.bboxMax = viewI16(data, off, k*Dims)
	off = align64(off + k*Dims*2)
	ix.blocks = viewI16(data, off, nb*Dims*8)
	off = align64(off + nb*Dims*8*2)
	ix.labels = data[off : off+nb*8]
	off += nb * 8
	if off > size {
		return nil, fmt.Errorf("index sections overrun file (%d > %d)", off, size)
	}
	return ix, nil
}
