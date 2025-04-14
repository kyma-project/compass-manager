package apperrors

import (
	"fmt"
)

type ErrReason string
type ErrComponent string

const (
	ErrCompassManager        ErrComponent = "compass manager"
	ErrCompassDirectorClient ErrComponent = "compass director client"
	ErrCompassDirector       ErrComponent = "compass director"
	ErrMpsOAuth2             ErrComponent = "mps oauth2"
)

const (
	ErrCompassManagerInternal ErrReason = "err_compass_manager_internal"

	ErrDirectorNilResponse            ErrReason = "err_director_nil_response"
	ErrDirectorRuntimeIDMismatch      ErrReason = "err_director_runtime_id_mismatch"
	ErrDirectorClientGraphqlizer      ErrReason = "err_director_client_graphqlizer"
	ErrDirectorRuntimeIDInvalidFormat ErrReason = "err_director_runtime_id_invalid_format"
)

type ErrCode int
type CauseCode int

const (
	CodeBadGateway ErrCode = 502
	CodeInternal   ErrCode = 500
	CodeExternal   ErrCode = 501
	CodeForbidden  ErrCode = 403
	CodeBadRequest ErrCode = 400
	CodeNotFound   ErrCode = 404
)

const (
	Unknown               CauseCode = 10
	GlobalAccountNotFound CauseCode = 11
	RuntimeNotFound       CauseCode = 20
)

type AppError interface {
	Append(string, ...interface{}) AppError
	SetReason(ErrReason) AppError
	SetComponent(ErrComponent) AppError

	Code() ErrCode
	Cause() CauseCode
	Component() ErrComponent
	Reason() ErrReason
	Error() string
}

type appError struct {
	code         ErrCode
	internalCode CauseCode
	reason       ErrReason
	component    ErrComponent
	message      string
}

func errorf(code ErrCode, cause CauseCode, format string, a ...interface{}) AppError {
	return appError{code: code, internalCode: cause, message: fmt.Sprintf(format, a...)}
}

func BadGateway(message string) AppError {
	return appError{code: CodeBadGateway, internalCode: Unknown, message: message}
}

func Internalf(formatted string, a ...interface{}) AppError {
	return errorf(CodeInternal, Unknown, formatted, a...)
}

func Internal(message string) AppError {
	return appError{code: CodeInternal, internalCode: Unknown, message: message}
}

func External(formatted string, a ...interface{}) AppError {
	return errorf(CodeExternal, Unknown, formatted, a...)
}

func Forbidden(formatted string, a ...interface{}) AppError {
	return errorf(CodeForbidden, Unknown, formatted, a...)
}

func BadRequest(message string) AppError {
	return appError{code: CodeBadRequest, internalCode: Unknown, message: message}
}

func NotFound(message string) AppError {
	return appError{code: CodeNotFound, internalCode: RuntimeNotFound, message: message}
}

func InvalidGlobalAccount(message string) AppError {
	return appError{code: CodeBadRequest, internalCode: GlobalAccountNotFound, message: message}
}

func (ae appError) Append(additionalFormat string, a ...interface{}) AppError {
	format := additionalFormat + ", " + ae.message
	ae.message = fmt.Sprintf(format, a...)

	return ae
}

func (ae appError) SetReason(reason ErrReason) AppError {
	ae.reason = reason
	return ae
}

func (ae appError) SetComponent(comp ErrComponent) AppError {
	ae.component = comp
	return ae
}

func (ae appError) Code() ErrCode {
	return ae.code
}

func (ae appError) Error() string {
	return ae.message
}

func (ae appError) Cause() CauseCode {
	return ae.internalCode
}

func (ae appError) Component() ErrComponent {
	if ae.component == "" {
		return ErrCompassManager
	}
	return ae.component
}

func (ae appError) Reason() ErrReason {
	if (ae.component == "" || ae.component == ErrCompassManager) && ae.reason == "" {
		return ErrCompassManagerInternal
	}
	return ae.reason
}
