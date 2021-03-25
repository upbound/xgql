package resolvers

func getLimit(limit *int, len int) int {
	if limit == nil || *limit > len {
		return len
	}
	return *limit
}
