package errors

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/equinor/radix-job-scheduler/models/common"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type APIStatus interface {
	Status() *common.Status
}

type StatusError struct {
	ErrStatus common.Status
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
func (e *StatusError) Status() *common.Status {
	return &e.ErrStatus
}

func NewNotFound(kind, name string) *StatusError {
	return &StatusError{
		common.Status{
			Status:  common.StatusFailure,
			Reason:  common.StatusReasonNotFound,
			Code:    http.StatusNotFound,
			Message: NotFoundMessage(kind, name),
		},
	}
}

func NewInvalid(name string) *StatusError {
	return &StatusError{
		common.Status{
			Status:  common.StatusFailure,
			Reason:  common.StatusReasonInvalid,
			Code:    http.StatusUnprocessableEntity,
			Message: InvalidMessage(name),
		},
	}
}

func NewUnknown(err error) *StatusError {
	return &StatusError{
		common.Status{
			Status:  common.StatusFailure,
			Reason:  common.StatusReasonUnknown,
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

func ReasonForError(err error) common.StatusReason {
	switch t := err.(type) {
	case APIStatus:
		return t.Status().Reason
	default:
		return common.StatusReasonUnknown
	}
}
