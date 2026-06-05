package main

import "math"

const fastNProbe = 16

type top5 struct {
	dist  [5]int64
	label [5]uint8
}

func (t *top5) reset() {
	for i := 0; i < 5; i++ {
		t.dist[i] = math.MaxInt64
		t.label[i] = 0
	}
}

func (t *top5) worst() int64 {
	w := t.dist[0]
	for i := 1; i < 5; i++ {
		if t.dist[i] > w {
			w = t.dist[i]
		}
	}
	return w
}

func (t *top5) insert(d int64, lab uint8) {
	wi := 0
	for i := 1; i < 5; i++ {
		if t.dist[i] > t.dist[wi] {
			wi = i
		}
	}
	if d < t.dist[wi] {
		t.dist[wi] = d
		t.label[wi] = lab
	}
}

func (t *top5) fraudCount() uint8 {
	var c uint8
	for i := 0; i < 5; i++ {
		c += t.label[i]
	}
	return c
}

type scratch struct {
	centDist []float32
	picked   []int32
	scanned  []uint64
	top      top5
	bsum     [8]float32
}

func newScratch(k int) *scratch {
	return &scratch{
		centDist: make([]float32, k),
		picked:   make([]int32, fastNProbe),
		scanned:  make([]uint64, (k+63)/64),
	}
}

func (ix *Index) fraudCount(qf *[Dims]float32, qi *[Dims]int16, sc *scratch) uint8 {
	ix.scoreCentroids(qf, sc.centDist)
	pickTopN(sc.centDist, sc.picked)

	sc.top.reset()
	for i := range sc.scanned {
		sc.scanned[i] = 0
	}

	for _, c := range sc.picked {
		if c < 0 {
			break
		}
		ix.scanCluster(int(c), qf, qi, sc)
		sc.scanned[c>>6] |= 1 << (uint(c) & 63)
	}

	worst := sc.top.worst()
	for c := 0; c < ix.K; c++ {
		if sc.scanned[c>>6]&(1<<(uint(c)&63)) != 0 {
			continue
		}
		if ix.aabbLB(qi, c) >= worst {
			continue
		}
		ix.scanCluster(c, qf, qi, sc)
		worst = sc.top.worst()
	}

	return sc.top.fraudCount()
}

func (ix *Index) scoreCentroids(q *[Dims]float32, out []float32) {
	for c := 0; c < ix.K; c++ {
		base := c * Dims
		var s float32
		for d := 0; d < Dims; d++ {
			diff := q[d] - ix.centroids[base+d]
			s += diff * diff
		}
		out[c] = s
	}
}

func pickTopN(dist []float32, out []int32) {
	for i := range out {
		out[i] = -1
	}
	for i := 0; i < len(out); i++ {
		best := int32(-1)
		var bd float32
		for c := 0; c < len(dist); c++ {
			taken := false
			for j := 0; j < i; j++ {
				if out[j] == int32(c) {
					taken = true
					break
				}
			}
			if taken {
				continue
			}
			if best < 0 || dist[c] < bd {
				best = int32(c)
				bd = dist[c]
			}
		}
		out[i] = best
	}
}

func (ix *Index) aabbLB(q *[Dims]int16, c int) int64 {
	base := c * Dims
	var s int64
	for d := 0; d < Dims; d++ {
		v := int64(q[d])
		mn := int64(ix.bboxMin[base+d])
		mx := int64(ix.bboxMax[base+d])
		if v < mn {
			diff := mn - v
			s += diff * diff
		} else if v > mx {
			diff := v - mx
			s += diff * diff
		}
	}
	return s
}

func (ix *Index) scanCluster(c int, qf *[Dims]float32, qi *[Dims]int16, sc *scratch) {
	nVec := int(ix.counts[c])
	if nVec == 0 {
		return
	}
	startBlk := int(ix.offsets[c])
	endBlk := int(ix.offsets[c+1])
	for b, vi := startBlk, 0; b < endBlk; b, vi = b+1, vi+8 {
		worst := sc.top.worst()
		var wf float32
		if worst >= math.MaxInt64 {
			wf = math.MaxFloat32
		} else {
			wf = float32(worst)*1.0001 + 64
		}
		blk := &ix.blocks[b*Dims*8]
		if !scanBlock8(qf, blk, wf, &sc.bsum) {
			continue
		}
		valid := nVec - vi
		if valid > 8 {
			valid = 8
		}
		labelBase := b * 8
		for l := 0; l < valid; l++ {
			if sc.bsum[l] >= wf {
				continue
			}
			d := exactDist(qi, blk, l)
			sc.top.insert(d, ix.labels[labelBase+l])
		}
	}
}
