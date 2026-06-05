package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestRealDataExactness(t *testing.T) {
	refsPath := os.Getenv("REFS_PATH")
	payPath := os.Getenv("PAYLOADS_PATH")
	if refsPath == "" || payPath == "" {
		t.Skip("set REFS_PATH and PAYLOADS_PATH to run")
	}

	vecs, labels, n, err := loadRefs(refsPath)
	if err != nil {
		t.Fatal(err)
	}
	qrefs := make([]int16, n*Dims)
	for i := 0; i < n*Dims; i++ {
		qrefs[i] = quantizeRef(vecs[i])
	}
	ix := buildArrays(vecs, labels, n)
	sc := newScratch(ix.K)

	raw, err := os.ReadFile(payPath)
	if err != nil {
		t.Fatal(err)
	}
	var payloads []json.RawMessage
	if err := json.Unmarshal(raw, &payloads); err != nil {
		t.Fatal(err)
	}

	mismatches := 0
	for pi, p := range payloads {
		var cb bytes.Buffer
		if err := json.Compact(&cb, p); err != nil {
			t.Fatal(err)
		}
		body := cb.Bytes()
		var qi [Dims]int16
		if !vectorize(body, &qi) {
			t.Fatalf("payload %d: vectorize failed", pi)
		}
		var qf [Dims]float32
		for d := 0; d < Dims; d++ {
			qf[d] = float32(qi[d])
		}
		got := ix.fraudCount(&qf, &qi, sc)
		want := bruteForce(qrefs, labels, n, &qi)
		if got != want {
			mismatches++
			t.Errorf("payload %d: ivf=%d brute=%d", pi, got, want)
		}
	}
	t.Logf("checked %d payloads, %d mismatches", len(payloads), mismatches)
}
