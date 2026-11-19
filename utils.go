package bigcache

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func convertMBToBytes(value int) int {
	const maxInt = int(^uint(0) >> 1)
	if value > maxInt/(1024*1024) {
		return maxInt
	}
	return value * 1024 * 1024
}

func isPowerOfTwo(number int) bool {
	return (number != 0) && (number&(number-1)) == 0
}
