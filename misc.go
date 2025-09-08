package xpweb

// ptr is a generic function which returns a pointer to the specified object.
func ptr[T any](v T) *T {
	return &v
}
