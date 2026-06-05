package main

import "unsafe"

func scanBlock8(q *[Dims]float32, block *int16, worst float32, sum *[8]float32) bool {
	b := (*[Dims * 8]int16)(unsafe.Pointer(block))
	for l := 0; l < 8; l++ {
		sum[l] = 0
	}
	for d := 0; d < Dims; d++ {
		qd := q[d]
		base := d * 8
		for l := 0; l < 8; l++ {
			diff := qd - float32(b[base+l])
			sum[l] += diff * diff
		}
		if d == 7 {
			alive := false
			for l := 0; l < 8; l++ {
				if sum[l] < worst {
					alive = true
					break
				}
			}
			if !alive {
				return false
			}
		}
	}
	for l := 0; l < 8; l++ {
		if sum[l] < worst {
			return true
		}
	}
	return false
}

func exactDist(q *[Dims]int16, block *int16, lane int) int64 {
	b := (*[Dims * 8]int16)(unsafe.Pointer(block))
	var s int64
	for d := 0; d < Dims; d++ {
		diff := int64(q[d]) - int64(b[d*8+lane])
		s += diff * diff
	}
	return s
}
