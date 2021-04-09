package testutils

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/router"
)

type ControllerTestUtils struct {
	controllers []models.Controller
}

func New(controllers ...models.Controller) ControllerTestUtils {
	return ControllerTestUtils{
		controllers: controllers,
	}
}

// ExecuteRequest Helper method to issue a http request
func (ctrl *ControllerTestUtils) ExecuteRequest(method, path string) <-chan *http.Response {
	return ctrl.ExecuteRequestWithBody(method, path, nil)
}

// ExecuteRequest Helper method to issue a http request
func (ctrl *ControllerTestUtils) ExecuteRequestWithBody(method, path string, body interface{}) <-chan *http.Response {
	responseChan := make(chan *http.Response)

	go func() {
		var reader io.Reader

		if body != nil {
			payload, _ := json.Marshal(body)
			reader = bytes.NewReader(payload)
		}

		router := router.NewServer(models.NewEnv(), ctrl.controllers...)
		server := httptest.NewServer(router)
		defer server.Close()
		url := buildUrlFromServer(server, path)
		request, _ := http.NewRequest(method, url, reader)
		response, _ := http.DefaultClient.Do(request)
		responseChan <- response
		close(responseChan)
	}()

	return responseChan
}

// GetResponseBody Gets response payload as type
func GetResponseBody(response *http.Response, target interface{}) error {
	body, _ := ioutil.ReadAll(response.Body)

	return json.Unmarshal(body, target)
}

func buildUrlFromServer(server *httptest.Server, path string) string {
	url, _ := url.Parse(server.URL)
	url.Path = path
	return url.String()
}
