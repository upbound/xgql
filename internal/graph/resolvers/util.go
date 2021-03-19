package resolvers

func getLimit(limit *int, len int) int {
	if limit == nil || *limit > len {
		return len
	}
	return *limit
}

func getIntPtr(i *int64) *int {
	if i == nil {
		return nil
	}

	out := int(*i)
	return &out
}
