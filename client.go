//go:generate go run gen_names.go

// Package xpweb provides client functionality for the X-Plane 12 web API.
//
// https://developer.x-plane.com/article/x-plane-web-api/
//
// All use cases will involve instantiating an XPClient object.
//
//	client := &webxp.XPClient{}
//
// The default target URL will be http://localhost:8086, however if X-Plane.exe was started with
// the web service running on a different port, e.g. via a --web_server_port=8088 argument, the
// appropriate URL may be specified during or after XPClient instantiation.
//
//	client := &webxp.XPClient{URL: url}
//
// To work with functions which take dataref names, the available datarefs should be loaded into
// the client object's dataref cache.
//
//	if err := client.LoadDatarefs(ctx); err != nil {
//		return err
//	}
//
// Getting dataref values provides a type-agnostic DatarefValue object which provides methods to
// extract the typed value.
//
// For example, fetching a string value:
//
//	acfNameVal, err := client.GetDatarefValue(ctx, "sim/aircraft/view/acf_ui_name")
//	if err != nil {
//		return err
//	}
//	fmt.Printf("Loaded Aircraft: %s\n", acfNameVal.GetStringValue())
//
// Will produce output like:
//
//	Loaded Aircraft: Cessna Skyhawk (G1000)
//
// Or, fetching other types of values...
//
//	numTanksVal, err := client.GetDatarefValue(ctx, "sim/aircraft/overflow/acf_num_tanks")
//	if err != nil {
//		return fmt.Errorf("GetDatarefValue(): %w", err)
//	}
//	numTanks := numTanksVal.GetIntValue()
//	fmt.Printf("Number of tanks: %d\n", numTanks)
//
//	fuelVal, err := client.GetDatarefValue(ctx, "sim/flightmodel/weight/m_fuel")
//	if err != nil {
//		return err
//	}
//	fuel := fuelVal.GetFloatArrayValue()
//
//	for idx := range numTanks {
//		fmt.Printf("Tank %d: %.3f\n", idx, fuel[idx])
//	}
//
// Would produce output like:
//
//	Number of tanks: 2
//	Tank 0: 2.481
//	Tank 1: 2.481
//
// Dataref values can be set in their entirety, for example, to reduce fuel in all tanks and apply
// the entire array of fuel values at once:
//
//	for idx, tankFuel := range fuel {
//		fuel[idx] = tankFuel / 2
//	}
//
//	err = client.SetDatarefValue(ctx, "sim/flightmodel/weight/m_fuel", fuel)
//	if err != nil {
//		return err
//	}
//
// Or, for array values, a single element at a specified index may have its value set, for example
// to halve the fuel in only the first tank:
//
//	err = client.SetDatarefElementValue(ctx, "sim/flightmodel/weight/m_fuel", 0, fuel[0] / 2)
//	if err != nil {
//		return err
//	}
//
// Consants of known dataref names are provided in the github.com/janeprather/xpweb/names/dataref
// package, and may help avoid repeating string literals and the risk of typos going undetected
// during lint/build.
//
//	acfNameVal, err := client.GetDatarefValue(ctx, dataref.SimAircraftView_acf_ui_name)
//
// Command activation requires specifying a duration for the command.  This can be a zero value for
// commands which are performed instantly, like turning a switch on, or for a set number of seconds
// for longer commands like starting an engine.
//
//	if err := client.ActivateCommand(ctx, "sim/engines/engage_starters", 2); err != nil {
//		return err
//	}
//
// Constants of known command names are provided in the github.com/janeprather/xpweb/names/command
// package, and may help avoid repeating string literals and the risk of typos going undetected
// during lint/build.
//
//	err := client.ActivateCommand(ctx, command.SimElectrical_battery_1_on, 0)
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

	commands     CommandsMap
	commandsLock sync.RWMutex
	datarefs     DatarefsMap
	datarefsLock sync.RWMutex
}

type CommandsMap map[string]*Command
type DatarefsMap map[string]*Dataref

// ErrorResponse is an error response received from the API.
type ErrorResponse struct {
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

// Error allows ErrorResponse to implement the error interface.
func (e ErrorResponse) Error() string {
	return e.ErrorMessage
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
		// attempt to unmarshal an error response body
		errorData, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("response from API: %s (unable to read response body)",
				resp.Status)
		}
		errorResp := &ErrorResponse{}
		err = json.Unmarshal(errorData, errorResp)
		if err != nil {
			return fmt.Errorf("response from API: %s (unable to unmarshal response body)",
				resp.Status)
		}

		// we were able to get a proper error object from the API, return it
		return errorResp
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
