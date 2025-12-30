package components

// getOrDefault returns the value if non-zero, otherwise default.
func getOrDefault(val int, def int) int {
	if val != 0 {
		return val
	}
	return def
}

// getOrDefaultFloat returns the value if non-zero, otherwise default.
func getOrDefaultFloat(val float32, def float32) float32 {
	if val != 0 {
		return val
	}
	return def
}
