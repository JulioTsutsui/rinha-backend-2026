package main

import (
	"math/rand"
	"testing"
)

func bruteForce(qrefs []int16, labels []uint8, n int, qi *[Dims]int16) uint8 {
	var t top5
	t.reset()
	for i := 0; i < n; i++ {
		var s int64
		for d := 0; d < Dims; d++ {
			diff := int64(qi[d]) - int64(qrefs[i*Dims+d])
			s += diff * diff
		}
		t.insert(s, labels[i])
	}
	return t.fraudCount()
}

func TestIVFMatchesBruteForce(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	const n = 4000
	vecs := make([]float64, n*Dims)
	labels := make([]uint8, n)
	for i := 0; i < n; i++ {
		hasLast := r.Intn(2) == 0
		for d := 0; d < Dims; d++ {
			if (d == 5 || d == 6) && !hasLast {
				vecs[i*Dims+d] = -1
			} else {
				vecs[i*Dims+d] = r.Float64()
			}
		}
		labels[i] = uint8(r.Intn(2))
	}

	qrefs := make([]int16, n*Dims)
	for i := 0; i < n*Dims; i++ {
		qrefs[i] = quantizeRef(vecs[i])
	}

	ix := buildArrays(vecs, labels, n)
	sc := newScratch(ix.K)

	for iter := 0; iter < 2000; iter++ {
		var qi [Dims]int16
		var qf [Dims]float32
		hasLast := r.Intn(2) == 0
		for d := 0; d < Dims; d++ {
			if (d == 5 || d == 6) && !hasLast {
				qi[d] = Sentinel
			} else {
				qi[d] = int16(r.Intn(Scale + 1))
			}
			qf[d] = float32(qi[d])
		}
		got := ix.fraudCount(&qf, &qi, sc)
		want := bruteForce(qrefs, labels, n, &qi)
		if got != want {
			t.Fatalf("iter %d: ivf=%d brute=%d", iter, got, want)
		}
	}
}

func TestVectorizeBasic(t *testing.T) {
	body := []byte(`{"id":"tx-1","transaction":{"amount":384.88,"installments":3,"requested_at":"2026-03-11T20:23:35Z"},"customer":{"avg_amount":769.76,"tx_count_24h":3,"known_merchants":["MERC-009","MERC-001"]},"merchant":{"id":"MERC-001","mcc":"5912","avg_amount":298.95},"terminal":{"is_online":false,"card_present":true,"km_from_home":13.7},"last_transaction":{"timestamp":"2026-03-11T14:58:35Z","km_from_current":18.8}}`)
	var out [Dims]int16
	if !vectorize(body, &out) {
		t.Fatal("vectorize failed")
	}
	if out[11] != 0 {
		t.Errorf("known merchant should give dim11=0, got %d", out[11])
	}
	if out[10] != Scale {
		t.Errorf("card_present true should give dim10=%d, got %d", Scale, out[10])
	}
	if out[9] != 0 {
		t.Errorf("is_online false should give dim9=0, got %d", out[9])
	}
	if out[12] != mccRiskTable[5912] {
		t.Errorf("mcc 5912 risk mismatch: got %d want %d", out[12], mccRiskTable[5912])
	}
	if out[5] == Sentinel || out[6] == Sentinel {
		t.Errorf("last_transaction present should not be sentinel")
	}
}

func TestVectorizeNoLastTx(t *testing.T) {
	body := []byte(`{"transaction":{"amount":100,"installments":1,"requested_at":"2026-03-11T20:23:35Z"},"customer":{"avg_amount":500,"tx_count_24h":1,"known_merchants":[]},"merchant":{"id":"M1","mcc":"5311","avg_amount":100},"terminal":{"is_online":true,"card_present":false,"km_from_home":1.0},"last_transaction":null}`)
	var out [Dims]int16
	if !vectorize(body, &out) {
		t.Fatal("vectorize failed")
	}
	if out[5] != Sentinel || out[6] != Sentinel {
		t.Errorf("null last_transaction should give sentinels, got %d %d", out[5], out[6])
	}
	if out[11] != Scale {
		t.Errorf("unknown merchant should give dim11=%d, got %d", Scale, out[11])
	}
}
