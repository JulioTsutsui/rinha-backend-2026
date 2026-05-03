package main

import (
	"math/rand"
	"testing"
)

func makeVec(n int, seed int64) []float32 {
	r := rand.New(rand.NewSource(seed))
	v := make([]float32, n)
	for i := range v {
		v[i] = r.Float32()
	}
	return v
}

var (
	benchA = makeVec(14, 1)
	benchB = makeVec(14, 2)
)

func BenchmarkDotProduct(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = DotProduct(benchA, benchB)
	}
}

func BenchmarkEuclideanDistance(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = EuclideanDistance(benchA, benchB)
	}
}

func BenchmarkCosineSimilarity(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = CosineSimilarity(benchA, benchB)
	}
}
