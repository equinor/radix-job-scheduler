package utils

import (
	"net/http"

	"github.com/equinor/radix-job-scheduler/models"
)

// RadixMiddleware The middleware between router and radix handler functions
type RadixMiddleware struct {
	path     string
	method   string
	handler  models.RadixHandlerFunc
	kubeUtil models.KubeUtil
}

func NewRadixMiddleware(path, method string, handler models.RadixHandlerFunc, kubeUtil models.KubeUtil) *RadixMiddleware {
	mw := &RadixMiddleware{
		path,
		method,
		handler,
		kubeUtil,
	}

	return mw
}

// Handle Wraps radix handler methods
func (mw *RadixMiddleware) Handle(w http.ResponseWriter, r *http.Request) {
	// w.Header().Add("Access-Control-Allow-Origin", "*")

	mw.handler(w, r, mw.kubeUtil)
}
