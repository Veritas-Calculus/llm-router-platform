package resolvers

func safeGQLInt(v int64) int {
	if v > 2147483647 {
		return 2147483647
	}
	if v < -2147483648 {
		return -2147483648
	}
	return int(v)
}
