package main

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"sync"
	"unsafe"
)

const buildIters = 15

type refRecord struct {
	Vector []float64 `json:"vector"`
	Label  string    `json:"label"`
}

func buildIndex(inPath, outPath string) error {
	vecs, labels, n, err := loadRefs(inPath)
	if err != nil {
		return err
	}
	fmt.Printf("parsed %d refs\n", n)
	ix := buildArrays(vecs, labels, n)
	fmt.Printf("built index: K=%d blocks=%d\n", ix.K, ix.NBlocks)
	if err := writeIndex(outPath, ix); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", outPath)
	return nil
}

func loadRefs(path string) ([]float64, []uint8, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, 0, err
	}
	defer f.Close()

	var rd io.Reader = bufio.NewReaderSize(f, 1<<20)
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, nil, 0, err
		}
		defer gz.Close()
		rd = bufio.NewReaderSize(gz, 1<<20)
	}

	dec := json.NewDecoder(rd)
	if _, err := dec.Token(); err != nil {
		return nil, nil, 0, err
	}
	var vecs []float64
	var labels []uint8
	n := 0
	for dec.More() {
		var rec refRecord
		if err := dec.Decode(&rec); err != nil {
			return nil, nil, 0, err
		}
		if len(rec.Vector) != Dims {
			return nil, nil, 0, fmt.Errorf("record %d: vector len %d, want %d", n, len(rec.Vector), Dims)
		}
		vecs = append(vecs, rec.Vector...)
		if rec.Label == "fraud" {
			labels = append(labels, 1)
		} else {
			labels = append(labels, 0)
		}
		n++
	}
	return vecs, labels, n, nil
}

func buildArrays(vecs []float64, labels []uint8, n int) *Index {
	k := n / 300
	if k < 64 {
		k = 64
	}
	if k > 4096 {
		k = 4096
	}
	if k > n {
		k = n
	}

	cent := kmeans(vecs, n, k, buildIters)
	assign := assignAll(vecs, cent, n, k)

	counts := make([]uint32, k)
	for _, c := range assign {
		counts[c]++
	}

	offsets := make([]uint32, k+1)
	for c := 0; c < k; c++ {
		nb := (int(counts[c]) + 7) / 8
		offsets[c+1] = offsets[c] + uint32(nb)
	}
	nBlocks := int(offsets[k])

	blocks := make([]int16, nBlocks*Dims*8)
	blabels := make([]uint8, nBlocks*8)
	cursor := make([]int, k)
	for i := 0; i < n; i++ {
		c := assign[i]
		pos := cursor[c]
		cursor[c]++
		blk := int(offsets[c]) + pos/8
		lane := pos % 8
		bbase := blk * Dims * 8
		for d := 0; d < Dims; d++ {
			blocks[bbase+d*8+lane] = quantizeRef(vecs[i*Dims+d])
		}
		blabels[blk*8+lane] = labels[i]
	}

	bboxMin := make([]int16, k*Dims)
	bboxMax := make([]int16, k*Dims)
	for c := 0; c < k; c++ {
		for d := 0; d < Dims; d++ {
			bboxMin[c*Dims+d] = math.MaxInt16
			bboxMax[c*Dims+d] = math.MinInt16
		}
		nVec := int(counts[c])
		sb := int(offsets[c])
		for vi := 0; vi < nVec; vi++ {
			blk := sb + vi/8
			lane := vi % 8
			bbase := blk * Dims * 8
			for d := 0; d < Dims; d++ {
				v := blocks[bbase+d*8+lane]
				if v < bboxMin[c*Dims+d] {
					bboxMin[c*Dims+d] = v
				}
				if v > bboxMax[c*Dims+d] {
					bboxMax[c*Dims+d] = v
				}
			}
		}
	}

	return &Index{
		N:         n,
		K:         k,
		NBlocks:   nBlocks,
		centroids: cent,
		offsets:   offsets,
		counts:    counts,
		bboxMin:   bboxMin,
		bboxMax:   bboxMax,
		blocks:    blocks,
		labels:    blabels,
	}
}

func kmeans(vecs []float64, n, k, iters int) []float32 {
	sampleN := n
	if sampleN > 100000 {
		sampleN = 100000
	}
	r := rand.New(rand.NewSource(42))
	perm := r.Perm(n)
	idx := perm[:sampleN]

	cent := make([]float64, k*Dims)
	for i := 0; i < k; i++ {
		copy(cent[i*Dims:(i+1)*Dims], vecs[idx[i]*Dims:(idx[i]+1)*Dims])
	}

	assign := make([]int, sampleN)
	sums := make([]float64, k*Dims)
	cnts := make([]int, k)
	for it := 0; it < iters; it++ {
		for si, gi := range idx {
			assign[si] = nearestF64(vecs[gi*Dims:(gi+1)*Dims], cent, k)
		}
		for i := range sums {
			sums[i] = 0
		}
		for i := range cnts {
			cnts[i] = 0
		}
		for si, gi := range idx {
			c := assign[si]
			cnts[c]++
			for d := 0; d < Dims; d++ {
				sums[c*Dims+d] += vecs[gi*Dims+d]
			}
		}
		for c := 0; c < k; c++ {
			if cnts[c] == 0 {
				continue
			}
			inv := 1.0 / float64(cnts[c])
			for d := 0; d < Dims; d++ {
				cent[c*Dims+d] = sums[c*Dims+d] * inv
			}
		}
	}

	out := make([]float32, k*Dims)
	for i := range out {
		out[i] = float32(cent[i])
	}
	return out
}

func nearestF64(v, cent []float64, k int) int {
	best := 0
	bd := math.MaxFloat64
	for c := 0; c < k; c++ {
		var s float64
		base := c * Dims
		for d := 0; d < Dims; d++ {
			diff := v[d] - cent[base+d]
			s += diff * diff
		}
		if s < bd {
			bd = s
			best = c
		}
	}
	return best
}

func assignAll(vecs []float64, cent []float32, n, k int) []int32 {
	out := make([]int32, n)
	workers := runtime.NumCPU()
	chunk := (n + workers - 1) / workers
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		lo := w * chunk
		hi := lo + chunk
		if hi > n {
			hi = n
		}
		if lo >= hi {
			break
		}
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			for i := lo; i < hi; i++ {
				best := int32(0)
				bd := float32(math.MaxFloat32)
				for c := 0; c < k; c++ {
					var s float32
					base := c * Dims
					for d := 0; d < Dims; d++ {
						diff := float32(vecs[i*Dims+d]) - cent[base+d]
						s += diff * diff
					}
					if s < bd {
						bd = s
						best = int32(c)
					}
				}
				out[i] = best
			}
		}(lo, hi)
	}
	wg.Wait()
	return out
}

func writeIndex(path string, ix *Index) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(f, 1<<20)

	var hdr [64]byte
	copy(hdr[:8], indexMagic)
	binary.LittleEndian.PutUint32(hdr[8:12], uint32(ix.N))
	binary.LittleEndian.PutUint32(hdr[12:16], uint32(ix.K))
	binary.LittleEndian.PutUint32(hdr[16:20], uint32(ix.NBlocks))

	writeSec(w, hdr[:])
	writeSec(w, f32bytes(ix.centroids))
	writeSec(w, u32bytes(ix.offsets))
	writeSec(w, u32bytes(ix.counts))
	writeSec(w, i16bytes(ix.bboxMin))
	writeSec(w, i16bytes(ix.bboxMax))
	writeSec(w, i16bytes(ix.blocks))
	writeSec(w, ix.labels)

	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func writeSec(w *bufio.Writer, b []byte) {
	w.Write(b)
	if pad := (64 - len(b)%64) % 64; pad > 0 {
		var z [64]byte
		w.Write(z[:pad])
	}
}

func i16bytes(s []int16) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&s[0])), len(s)*2)
}
func u32bytes(s []uint32) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&s[0])), len(s)*4)
}
func f32bytes(s []float32) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&s[0])), len(s)*4)
}
