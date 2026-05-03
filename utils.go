package main

func Limitar(value float32) float32 {
	if value < 0.0 {
		return 0.0
	}

	if value > 1.0 {
		return 1.0
	}

	return value
}

func Bool2Float(value bool) float32 {
	if value == true {
		return 1
	}

	return 0
}
