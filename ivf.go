package main

import (
	"math"
	"math/rand"
)

const (
	IVFK         = 256
	KMeansIters  = 10
	KMeansSample = 32768
	Nprobe       = 4
)

// IVFIndex partitions the reference set into K clusters via k-means and lets
// queries scan only the most promising clusters. Correctness is preserved by
// bounding-box repair: any cluster whose bbox-lower-bound distance to the
// query is already worse than the current top-K worst is provably safe to skip.
type IVFIndex struct {
	N         int
	K         int
	Centroids []float32 // K * VecDim
	BBoxMin   []float32 // K * VecDim
	BBoxMax   []float32 // K * VecDim
	Offset    []int32   // K+1, cluster c spans refs [Offset[c], Offset[c+1])
	Refs      []float32 // N * VecDim, reordered by cluster
	Labels    []string  // N, reordered by cluster
}

// BuildIVF consumes the loose float references and produces a clustered index.
// The caller can drop the original slices afterwards.
func BuildIVF(flat []float32, labels []string, n int) *IVFIndex {
	centroids := trainKMeans(flat, n, IVFK, KMeansSample, KMeansIters)

	assign := make([]int32, n)
	counts := make([]int32, IVFK)
	for i := 0; i < n; i++ {
		c := nearestCentroidF32(flat[i*VecDim:(i+1)*VecDim], centroids, IVFK)
		assign[i] = int32(c)
		counts[c]++
	}

	offset := make([]int32, IVFK+1)
	for c := 0; c < IVFK; c++ {
		offset[c+1] = offset[c] + counts[c]
	}

	refs := make([]float32, n*VecDim)
	newLabels := make([]string, n)
	cursor := make([]int32, IVFK)
	for i := 0; i < n; i++ {
		c := assign[i]
		dst := offset[c] + cursor[c]
		cursor[c]++
		copy(refs[int(dst)*VecDim:(int(dst)+1)*VecDim], flat[i*VecDim:(i+1)*VecDim])
		newLabels[dst] = labels[i]
	}

	bbMin := make([]float32, IVFK*VecDim)
	bbMax := make([]float32, IVFK*VecDim)
	for c := 0; c < IVFK; c++ {
		start, end := int(offset[c]), int(offset[c+1])
		base := c * VecDim
		if start == end {
			continue
		}
		for d := 0; d < VecDim; d++ {
			bbMin[base+d] = math.MaxFloat32
			bbMax[base+d] = -math.MaxFloat32
		}
		for i := start; i < end; i++ {
			for d := 0; d < VecDim; d++ {
				v := refs[i*VecDim+d]
				if v < bbMin[base+d] {
					bbMin[base+d] = v
				}
				if v > bbMax[base+d] {
					bbMax[base+d] = v
				}
			}
		}
	}

	return &IVFIndex{
		N: n, K: IVFK,
		Centroids: centroids,
		BBoxMin:   bbMin,
		BBoxMax:   bbMax,
		Offset:    offset,
		Refs:      refs,
		Labels:    newLabels,
	}
}

func trainKMeans(flat []float32, n, k, sampleN, iters int) []float32 {
	if sampleN > n {
		sampleN = n
	}
	r := rand.New(rand.NewSource(42))
	perm := r.Perm(n)
	sampleIdx := perm[:sampleN]

	centroids := make([]float32, k*VecDim)
	for i := 0; i < k; i++ {
		src := sampleIdx[i]
		copy(centroids[i*VecDim:(i+1)*VecDim], flat[src*VecDim:(src+1)*VecDim])
	}

	assign := make([]int, sampleN)
	newCent := make([]float32, k*VecDim)
	counts := make([]int, k)
	for it := 0; it < iters; it++ {
		for i, idx := range sampleIdx {
			assign[i] = nearestCentroidF32(flat[idx*VecDim:(idx+1)*VecDim], centroids, k)
		}
		for i := range newCent {
			newCent[i] = 0
		}
		for i := range counts {
			counts[i] = 0
		}
		for i, idx := range sampleIdx {
			c := assign[i]
			counts[c]++
			for d := 0; d < VecDim; d++ {
				newCent[c*VecDim+d] += flat[idx*VecDim+d]
			}
		}
		for c := 0; c < k; c++ {
			if counts[c] == 0 {
				continue // keep old centroid
			}
			inv := 1.0 / float32(counts[c])
			for d := 0; d < VecDim; d++ {
				centroids[c*VecDim+d] = newCent[c*VecDim+d] * inv
			}
		}
	}
	return centroids
}

func nearestCentroidF32(v, centroids []float32, k int) int {
	best := 0
	bestD := float32(math.MaxFloat32)
	for c := 0; c < k; c++ {
		var sum float32
		base := c * VecDim
		for d := 0; d < VecDim; d++ {
			diff := v[d] - centroids[base+d]
			sum += diff * diff
		}
		if sum < bestD {
			bestD = sum
			best = c
		}
	}
	return best
}

// topNprobe writes the indices of the nprobe nearest centroids (by float32
// squared distance) into out[:nprobe]. Implementation is a small partial sort —
// fine because nprobe is tiny (4) and K is small (256).
func topNprobe(qf, centroids []float32, k, nprobe int, out []int) {
	type cd struct {
		c int
		d float32
	}
	dists := make([]cd, k)
	for c := 0; c < k; c++ {
		var sum float32
		base := c * VecDim
		for d := 0; d < VecDim; d++ {
			diff := qf[d] - centroids[base+d]
			sum += diff * diff
		}
		dists[c] = cd{c, sum}
	}
	// selection sort over the first nprobe positions
	for i := 0; i < nprobe; i++ {
		min := i
		for j := i + 1; j < k; j++ {
			if dists[j].d < dists[min].d {
				min = j
			}
		}
		dists[i], dists[min] = dists[min], dists[i]
		out[i] = dists[i].c
	}
}

// bboxLowerBound returns a provable lower bound on the squared euclidean
// distance from q to ANY point inside the axis-aligned box [bbMin, bbMax].
// If this lower bound is already >= the current top-K worst distance, we can
// skip the entire cluster without missing a better neighbor.
func bboxLowerBound(q, bbMin, bbMax []float32) float32 {
	var sum float32
	for d := 0; d < VecDim; d++ {
		var diff float32
		if q[d] < bbMin[d] {
			diff = bbMin[d] - q[d]
		} else if q[d] > bbMax[d] {
			diff = q[d] - bbMax[d]
		} else {
			continue
		}
		sum += diff * diff
	}
	return sum
}

type ivfHit struct {
	dist float32
	id   int32
}

// SearchTopK returns labels of the topK nearest references (by squared
// euclidean) using IVF + bbox repair. Result is identical to a brute-force scan.
func (idx *IVFIndex) SearchTopK(qf []float32, topK int) []string {
	top := make([]ivfHit, 0, topK)
	worstIdx := 0

	scanCluster := func(c int) {
		start, end := int(idx.Offset[c]), int(idx.Offset[c+1])
		for i := start; i < end; i++ {
			refVec := idx.Refs[i*VecDim : (i+1)*VecDim]
			d := EuclideanDistance(qf, refVec)
			if len(top) < topK {
				top = append(top, ivfHit{d, int32(i)})
				if len(top) == topK {
					worstIdx = 0
					for j := 1; j < topK; j++ {
						if top[j].dist > top[worstIdx].dist {
							worstIdx = j
						}
					}
				}
				continue
			}
			if d >= top[worstIdx].dist {
				continue
			}
			top[worstIdx] = ivfHit{d, int32(i)}
			worstIdx = 0
			for j := 1; j < topK; j++ {
				if top[j].dist > top[worstIdx].dist {
					worstIdx = j
				}
			}
		}
	}

	probes := make([]int, Nprobe)
	topNprobe(qf, idx.Centroids, idx.K, Nprobe, probes)

	probed := make([]bool, idx.K)
	for _, c := range probes {
		probed[c] = true
		scanCluster(c)
	}

	for c := 0; c < idx.K; c++ {
		if probed[c] {
			continue
		}
		if len(top) == topK {
			lb := bboxLowerBound(qf, idx.BBoxMin[c*VecDim:(c+1)*VecDim], idx.BBoxMax[c*VecDim:(c+1)*VecDim])
			if lb >= top[worstIdx].dist {
				continue
			}
		}
		scanCluster(c)
	}

	labels := make([]string, len(top))
	for i, e := range top {
		labels[i] = idx.Labels[e.id]
	}
	return labels
}
