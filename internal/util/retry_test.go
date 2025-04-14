package util

import (
	"testing"

	"github.com/kyma-project/compass-manager/internal/apperrors"
	"github.com/stretchr/testify/require"
)

func TestRetryOnError(t *testing.T) {
	t.Run("should retry function on error", func(t *testing.T) {
		// given
		tester := tester{errReturned: false}

		// when
		err := RetryOnError(1, 2, "function call returned error: %s", tester.testFunction)

		// then
		require.NoError(t, err)
	})
}

type tester struct {
	errReturned bool
}

// testFunction returns error on first call and nil on subsequent calls
func (t *tester) testFunction() apperrors.AppError {
	if t.errReturned {
		return nil
	}
	t.errReturned = true
	return apperrors.Internalf("some test error")
}
