package llm

import "errors"

// errorsIs is a tiny indirection so provider files read consistently and the
// dependency on errors stays in one place.
func errorsIs(err, target error) bool { return errors.Is(err, target) }
