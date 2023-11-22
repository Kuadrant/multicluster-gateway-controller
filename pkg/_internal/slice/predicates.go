package slice

func EqualsTo[T comparable](x T) func(T) bool {
	return func(y T) bool {
		return x == y
	}
}

func Not[T any](predicate func(T) bool) func(T) bool {
	return func(x T) bool {
		return !predicate(x)
	}
}
