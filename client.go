package xpweb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"sync"
)

const defaultURLBase string = "http://localhost:8086"

// XPClient is an X-Plane Web API client.
type XPClient struct {
	// An optional URL.  If unspecified, http://localhost:8086 will be used.
	URL string
	// An optional http.RoundTripper which will be used to perform the HTTP requests.  If left
	// unspecified, the http.DefaultTransport will be used.
	Transport http.RoundTripper

	datarefs     DatarefsMap
	datarefsLock sync.RWMutex
}

type DatarefsMap map[string]*Dataref

// ErrorResponse is an error response received from the API.
type ErrorResponse struct {
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

// assertValues applies default values
func (xpc *XPClient) assertValues() {
	if xpc.URL == "" {
		xpc.URL = defaultURLBase
	}
	if xpc.Transport == nil {
		xpc.Transport = http.DefaultTransport
	}

	// trim any trailing / off the URL
	trailingSlashes := regexp.MustCompile("/+$")
	xpc.URL = trailingSlashes.ReplaceAllString(xpc.URL, "")
}

func (xpc *XPClient) RestRequest(
	ctx context.Context,
	method string,
	path string,
	bodyObj any,
	target any,
) error {
	xpc.assertValues()

	// prepare body payload
	var body io.Reader
	if bodyObj != nil {
		bodyData, err := json.Marshal(bodyObj)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewBuffer(bodyData)
	}

	url := xpc.URL + path

	// perform request
	request, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create new request: %w", err)
	}

	request.Header.Add("Accept", "application/json")
	if body != nil {
		request.Header.Add("Content-Type", "application/json")
	}

	client := &http.Client{Transport: xpc.Transport}

	resp, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// attempt to unmarshal an error
		var errorMessage string
		errorData, err := io.ReadAll(resp.Body)
		if err != nil {
			errorMessage = fmt.Sprintf("unable to read response body: %s", err.Error())
		} else {
			errorResp := &ErrorResponse{}
			err = json.Unmarshal(errorData, errorResp)
			if err != nil {
				errorMessage = "unable to unmarshal response body"
			} else {
				errorMessage = errorResp.ErrorMessage
			}
		}
		return fmt.Errorf("non-200 response from API: %s - %s", resp.Status, errorMessage)
	}

	if target != nil {
		bodyData, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("unable to read response body: %w", err)
		}

		err = json.Unmarshal(bodyData, &target)
		if err != nil {
			return fmt.Errorf("unable to unmarshal response into %s: %w",
				reflect.TypeOf(target).String(), err)
		}
	}

	return nil
}
