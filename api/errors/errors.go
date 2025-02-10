package errors

import (
	"fmt"
	"net/http"
)

type APIStatus interface {
	Status() *Status
}

var _ error = &StatusError{}

type StatusError struct {
	ErrStatus       Status
	UnderlyingError error
}

// Error implements the Error interface.
func (e *StatusError) Error() string {
	if e.UnderlyingError == nil {
		return e.ErrStatus.Message
	}
	return e.UnderlyingError.Error()
}

// Error implements the Error interface.
func (e *StatusError) Status() *Status {
	return &e.ErrStatus
}

func NotFoundMessage(resourceType, name string) string {
	return fmt.Sprintf("%s %s not found", resourceType, name)
}

func InvalidMessage(name, reason string) string {
	message := fmt.Sprintf("%s is invalid", name)
	if len(reason) > 0 {
		message = fmt.Sprintf("%s: %s", message, reason)
	}
	return message
}

func UnknownMessage(err error) string {
	return err.Error()
}

func NewBadRequestError(message string, underlyingError error) *StatusError {
	return &StatusError{
		ErrStatus: Status{
			Status:  StatusFailure,
			Reason:  StatusReasonBadRequest,
			Code:    http.StatusBadRequest,
			Message: message,
		},
		UnderlyingError: underlyingError,
	}
}

func NewNotFoundError(resourceType, name string, underlyingError error) *StatusError {
	return &StatusError{
		ErrStatus: Status{
			Status:  StatusFailure,
			Reason:  StatusReasonNotFound,
			Code:    http.StatusNotFound,
			Message: NotFoundMessage(resourceType, name),
		},
		UnderlyingError: underlyingError,
	}
}

func NewInvalidWithReason(name, reason string) *StatusError {
	return &StatusError{
		ErrStatus: Status{
			Status:  StatusFailure,
			Reason:  StatusReasonInvalid,
			Code:    http.StatusUnprocessableEntity,
			Message: InvalidMessage(name, reason),
		},
	}
}

func NewInvalid(name string) *StatusError {
	return &StatusError{
		ErrStatus: Status{
			Status:  StatusFailure,
			Reason:  StatusReasonInvalid,
			Code:    http.StatusUnprocessableEntity,
			Message: InvalidMessage(name, ""),
		},
	}
}

func NewInternalError(err error) *StatusError {
	return &StatusError{
		ErrStatus: Status{
			Status:  StatusFailure,
			Reason:  StatusReasonInternalError,
			Code:    http.StatusInternalServerError,
			Message: "Internal Server Error",
		},
		UnderlyingError: err,
	}
}

func NewFromError(err error) *StatusError {
	switch t := err.(type) {
	case *StatusError:
		return t
	// case k8sErrors.APIStatus:
	// 	return NewFromKubernetesAPIStatus(t)
	default:
		return NewInternalError(err)
	}
}

// func NewFromKubernetesAPIStatus(apiStatus k8sErrors.APIStatus) *StatusError {
// 	switch apiStatus.Status().Reason {
// 	case v1.StatusReasonNotFound:
// 		return NewNotFound(apiStatus.Status().Details.Kind, apiStatus.Status().Details.Name)
// 	case v1.StatusReasonInvalid:
// 		return NewInvalid(apiStatus.Status().Details.Name)
// 	default:
// 		return NewInternalError(errors.New(apiStatus.Status().Message))
// 	}
// }

func ReasonForError(err error) StatusReason {
	switch t := err.(type) {
	case APIStatus:
		return t.Status().Reason
	// case k8sErrors.APIStatus:
	// 	return NewFromError(err).Status().Reason
	default:
		return NewFromError(err).Status().Reason
	}
}
