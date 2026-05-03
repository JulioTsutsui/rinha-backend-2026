package main

import "math"

func VectorialSearch(a, b []float32, algo string) float32 {
	if algo == "euc" {
		return EuclideanDistance(a, b)
	}
	if algo == "cos" {
		return CosineSimilarity(a, b)
	}

	return DotProduct(a, b)
}

func DotProduct(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var sum float32
	for i := 0; i < len(a); i++ {
		sum += a[i] * b[i]
	}
	return sum
}

// EuclideanDistance returns the SQUARED euclidean distance.
// OPTIMIZATION: sqrt is omitted on purpose — top-K ranking by sum-of-squares
// is identical to ranking by sqrt(sum-of-squares), so we save 3M sqrt calls
// per request. If you ever need the true distance, take sqrt at the call site.
func EuclideanDistance(a, b []float32) float32 {
	if len(a) != len(b) {
		return -1
	}
	var sum float32
	for i := 0; i < len(a); i++ {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return sum
}

func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32

	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	normA = float32(math.Sqrt(float64(normA)))
	normB = float32(math.Sqrt(float64(normB)))

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (normA * normB)
}
