package errors

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/equinor/radix-job-scheduler/models"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type APIStatus interface {
	Status() *models.Status
}

type StatusError struct {
	ErrStatus models.Status
}

var _ error = &StatusError{}

func NotFoundMessage(kind, name string) string {
	return fmt.Sprintf("%s %s not found", kind, name)
}

func InvalidMessage(name string) string {
	return fmt.Sprintf("%s is invalid", name)
}

func UnknownMessage(err error) string {
	return err.Error()
}

// Error implements the Error interface.
func (e *StatusError) Error() string {
	return e.ErrStatus.Message
}

// Error implements the Error interface.
func (e *StatusError) Status() *models.Status {
	return &e.ErrStatus
}

func NewNotFound(kind, name string) *StatusError {
	return &StatusError{
		models.Status{
			Status:  models.StatusFailure,
			Reason:  models.StatusReasonNotFound,
			Code:    http.StatusNotFound,
			Message: NotFoundMessage(kind, name),
		},
	}
}

func NewInvalid(name string) *StatusError {
	return &StatusError{
		models.Status{
			Status:  models.StatusFailure,
			Reason:  models.StatusReasonInvalid,
			Code:    http.StatusUnprocessableEntity,
			Message: InvalidMessage(name),
		},
	}
}

func NewUnknown(err error) *StatusError {
	return &StatusError{
		models.Status{
			Status:  models.StatusFailure,
			Reason:  models.StatusReasonUnknown,
			Code:    http.StatusInternalServerError,
			Message: UnknownMessage(err),
		},
	}
}

func NewFromError(err error) *StatusError {
	switch t := err.(type) {
	case *StatusError:
		return t
	case k8sErrors.APIStatus:
		return NewFromKubernetesAPIStatus(t)
	default:
		return NewUnknown(err)
	}
}

func NewFromKubernetesAPIStatus(apiStatus k8sErrors.APIStatus) *StatusError {
	switch apiStatus.Status().Reason {
	case v1.StatusReasonNotFound:
		return NewNotFound(apiStatus.Status().Details.Kind, apiStatus.Status().Details.Name)
	case v1.StatusReasonInvalid:
		return NewInvalid(apiStatus.Status().Details.Name)
	default:
		return NewUnknown(errors.New(apiStatus.Status().Message))
	}
}

func ReasonForError(err error) models.StatusReason {
	switch t := err.(type) {
	case APIStatus:
		return t.Status().Reason
	default:
		return models.StatusReasonUnknown
	}
}
