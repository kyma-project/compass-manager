package apperrors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError(t *testing.T) {
	t.Run("should create error with proper code", func(t *testing.T) {
		assert.Equal(t, CodeInternal, Internal("error").Code())
		assert.Equal(t, CodeForbidden, Forbidden("error").Code())
		assert.Equal(t, CodeBadRequest, BadRequest("error").Code())
	})

	t.Run("should create error with simple message", func(t *testing.T) {
		assert.Equal(t, "error", Internal("error").Error())
		assert.Equal(t, "error", Forbidden("error").Error())
		assert.Equal(t, "error", BadRequest("error").Error())
	})

	t.Run("should create error with formatted message", func(t *testing.T) {
		assert.Equal(t, "code: 1, error: bug", Internalf("code: %d, error: %s", 1, "bug").Error())
		assert.Equal(t, "code: 1, error: bug", Forbidden("code: %d, error: %s", 1, "bug").Error())
	})

	t.Run("should append apperrors without changing error code", func(t *testing.T) {
		// given
		createdInternalErr := Internalf("Some Internal apperror, %s", "Some pkg err")
		createdForbiddenErr := Forbidden("Some Forbidden apperror, %s", "Some pkg err")

		// when
		appendedInternalErr := createdInternalErr.Append("Some additional message")
		appendedForbiddenErr := createdForbiddenErr.Append("Some additional message")

		// then
		assert.Equal(t, CodeInternal, appendedInternalErr.Code())
		assert.Equal(t, CodeForbidden, appendedForbiddenErr.Code())
	})

	t.Run("should append apperrors and chain messages correctly", func(t *testing.T) {
		// given
		createdInternalErr := Internalf("Some Internal apperror, %s", "Some pkg err")
		createdForbiddenErr := Forbidden("Some Forbidden apperror, %s", "Some pkg err")

		// when
		appendedInternalErr := createdInternalErr.Append("Some additional message: %s", "error")
		appendedForbiddenErr := createdForbiddenErr.Append("Some additional message: %s", "error")

		// then
		assert.Equal(t, "Some additional message: error, Some Internal apperror, Some pkg err", appendedInternalErr.Error())
		assert.Equal(t, "Some additional message: error, Some Forbidden apperror, Some pkg err", appendedForbiddenErr.Error())
	})
}
