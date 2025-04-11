package util

import (
	"testing"

	"github.com/kyma-project/compass-manager/internal/apperrors"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func CheckErrorType(t *testing.T, err error, errCode apperrors.ErrCode) {
	var appErr apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fail()
	}
	assert.Equal(t, appErr.Code(), errCode)
}
