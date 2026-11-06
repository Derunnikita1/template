package create_test

import "errors"

func containsError(err, target error) bool {
	return errors.Is(err, target)
}
