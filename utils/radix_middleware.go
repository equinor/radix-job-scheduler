package utils

import (
	"github.com/equinor/radix-job-scheduler/models"
	"net/http"
)

// RadixMiddleware The middleware between router and radix handler functions
type RadixMiddleware struct {
	path    string
	method  string
	handler models.RadixHandlerFunc
}

func NewRadixMiddleware(path, method string, handler models.RadixHandlerFunc) *RadixMiddleware {
	mw := &RadixMiddleware{
		path,
		method,
		handler,
	}

	return mw
}

// Handle Wraps radix handler methods
func (mw *RadixMiddleware) Handle(w http.ResponseWriter, r *http.Request) {
	mw.handler(w, r)
}
