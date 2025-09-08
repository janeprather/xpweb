//go:generate go run gen_names.go

// Package xpweb provides client functionality for the X-Plane 12 web API.
//
// https://developer.x-plane.com/article/x-plane-web-api/
//
// All use cases will involve instantiating an [Client] object with the NewClient function.  This
// may return an error if a bad URL has been provided.
//
//	client, err := webxp.NewClient(nil)
//
// The default target URL will be http://localhost:8086, however if X-Plane.exe was started with
// the web service running on a different port, e.g. via a --web_server_port=8088 argument, the
// appropriate URL may be specified during or after [Client] instantiation.  Additonally, a custom
// http.RoundTripper may be specified as the transport.
//
//	client, err := webxp.NewClient(&webxp.ClientConfig{
//		URL:       apiURL,
//		Transport: apiTransport,
//	})
//
// If the calling application will be establishing a websocket connection, then handlers for the
// result, command update, and dataref update messages received from the websocket service can be
// specified in the ClientConfig object as well.
//
//	handleResult = func(msg *WSMessageResult) {
//		output := fmt.Sprintf("request %d (%s) result: %v", msg.ReqID, msg.Req.Type, msg.Success)
//		if msg.ErrorMessage != "" {
//			output += fmt.Sprintf(" (%s)", msg.ErrorMessage)
//		}
//		fmt.Println(output)
//	}
//
//	handleDatarefUpdate := func(msg *xpweb.WSMessageDatarefUpdate) {
//		var drefUdates []string
//		for _, val := range msg.Data {
//			drefUdates = append(drefUdates, fmt.Sprintf("  %s: %v\n", val.Dataref.Name, val.Value))
//		}
//		fmt.Printf("dataref(s) update:\n%s", strings.Join(drefUdates, ""))
//	}
//
//	handleCommandUpdate := func(msg *xpweb.WSMessageCommandUpdate) {
//		var cmdUdates []string
//		for _, val := range msg.Data {
//			cmdUdates = append(cmdUdates, fmt.Sprintf("  %s: %v\n", val.Command.Name, val.IsActive))
//		}
//		fmt.Printf("command(s) update:\n%s", strings.Join(cmdUdates, ""))
//	}
//
//	client, err := webxp.NewClient(&webxp.ClientConfig{
//		CommandUpdateHandler: handleCommandUpdate,
//		DatarefUpdateHandler: handleDatarefUpdate,
//		ResultHandler:        handleResult,
//		URL:                  apiURL,
//		Transport:            apiTransport,
//	})
//
// After instantiation, and after any restart of the simulator, the cache of command and dataref
// values needs to be reloaded from the simulator.  The ID values for commands or datarefs are not
// guaranteed to remain unchanged from one simulator session to the next.
//
//	if err := client.LoadCache(ctx); err != nil {
//		return err
//	}
//
// A significant portion of this package is broken up into REST-specific and websocket-specific
// functionality.  While some of the more agnostic methods are available via the [Client] object,
// much of the API functions used by calling applications will be done using either the [WSClient]
// or [RESTClient] objects accessible via the [Client] object.
//
//	xpREST := client.REST
//	xpWS := client.WS
//
// [DatarefValue] objects returned from either the REST or websocket service are type-agnostic.
// They have a Value attribute of any type.  If the type of the dataref values is known, a suitable
// method may be used to return the value as the correct type.
//
// For example, fetching a string value:
//
//	acfNameVal, err := client.REST.GetDatarefValue(ctx, "sim/aircraft/view/acf_ui_name")
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
//	numTanksVal, err := client.REST.GetDatarefValue(ctx, "sim/aircraft/overflow/acf_num_tanks")
//	if err != nil {
//		return fmt.Errorf("GetDatarefValue(): %w", err)
//	}
//	numTanks := numTanksVal.GetIntValue()
//	fmt.Printf("Number of tanks: %d\n", numTanks)
//
//	fuelVal, err := client.REST.GetDatarefValue(ctx, "sim/flightmodel/weight/m_fuel")
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
//	err = client.REST.SetDatarefValue(ctx, "sim/flightmodel/weight/m_fuel", fuel)
//	if err != nil {
//		return err
//	}
//
// Or, for array values, a single element at a specified index may have its value set, for example
// to halve the fuel in only the first tank:
//
//	err = client.REST.SetDatarefElementValue(ctx, "sim/flightmodel/weight/m_fuel", 0, fuel[0] / 2)
//	if err != nil {
//		return err
//	}
//
// Consants of known dataref names are provided in the github.com/janeprather/xpweb/names/dataref
// package, and may help avoid repeating string literals and the risk of typos going undetected
// during lint/build.  Note that these will be msising values which are specific to third party
// aircraft and plugins.
//
//	acfNameVal, err := client.REST.GetDatarefValue(ctx, dataref.SimAircraftView_acf_ui_name)
//
// Command activation requires specifying a duration for the command.  This can be a zero value for
// commands which are performed instantly, like turning a switch on, or for a set number of seconds
// for longer commands like starting an engine.
//
//	if err := client.REST.ActivateCommand(ctx, "sim/engines/engage_starters", 2); err != nil {
//		return err
//	}
//
// Constants of known command names are provided in the github.com/janeprather/xpweb/names/command
// package, and may help avoid repeating string literals and the risk of typos going undetected
// during lint/build.  Note that these will be missing values which are specific to third party
// aircraft and plugins.
//
//	err := client.REST.ActivateCommand(ctx, command.SimElectrical_battery_1_on, 0)
//
// To start using the websocket service, establish a connection.
//
//	if err := client.WS.Connect(); err != nil {
//		return err
//	}
//	defer client.WS.Close()
//
// All outbound websocket requests are instantiated with [WSClient.NewReq].  One of several methods
// should be used to apply appropriate type and params to the request.
//
//   - [WSReq.CommandSetIsActive]
//   - [WSReq.CommandSubscribe]
//   - [WSReq.CommandUnsubscribe]
//   - [WSReq.CommandUnsubscribeAll]
//   - [WSReq.DatarefSet]
//   - [WSReq.DatarefSubscribe]
//   - [WSReq.DatarefUnsubscribe]
//   - [WSReq.DatarefUnsubscribeAll]
//
// The chained [WSReq] instantiation can be finished off with a [WSReq.Send] call to submit the
// request.
//
//	if err := xpWS.NewReq().DatarefSubscribe(
//		xpWS.NewDataref("sim/flightmodel/weight/m_fuel").WithIndexArray([]int{0, 1}),
//	).Send(); err != nil {
//		return err
//	}
package xpweb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"sync"
)

const defaultURLBase string = "http://localhost:8086"

// Client is an X-Plane Web API client.
type Client struct {
	REST *RESTClient
	WS   *WSClient

	transport http.RoundTripper

	commandsByID   commandsIDMap
	commandsByName commandsNameMap
	commandsLock   sync.RWMutex

	datarefsByID   datarefsIDMap
	datarefsByName datarefsNameMap
	datarefsLock   sync.RWMutex
}

// RestClient provides functions and attributes related to REST API operations.
type RESTClient struct {
	client *Client
	url    *url.URL
}

// ClientConfig is a structure which may optionall be passed to NewClient().
type ClientConfig struct {
	// An optional URL.  If unspecified, http://localhost:8086 will be used.
	URL string
	// An optional http.RoundTripper which will be used to perform the HTTP requests.  If left
	// unspecified, the http.DefaultTransport will be used.
	Transport http.RoundTripper
	// The handler function for command update messages received from the websocket service.
	CommandUpdateHandler CommandUpdateHandler
	// The handler function for dataref update messages received from the websocket service.
	DatarefUpdateHandler DatarefUpdateHandler
	// The handler function for result messages received from the websocket service.
	ResultHandler ResultHandler
}

type commandsIDMap map[uint64]*Command
type commandsNameMap map[string]*Command
type datarefsIDMap map[uint64]*Dataref
type datarefsNameMap map[string]*Dataref

// ErrorResponse is an error response received from the API.
type ErrorResponse struct {
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

// Error allows ErrorResponse to implement the error interface.
func (e ErrorResponse) Error() string {
	return e.ErrorMessage
}

// NewClient instantiates and returns a pointer to a new [Client] object.
func NewClient(config *ClientConfig) (client *Client, err error) {
	// defaults
	apiURL := defaultURLBase
	transport := http.DefaultTransport

	// config-specified values
	if config != nil {
		if config.URL != "" {
			apiURL = config.URL
		}
		if config.Transport != nil {
			transport = config.Transport
		}
	}

	// trim any trailing / off the URL
	trailingSlashes := regexp.MustCompile("/+$")
	apiURL = trailingSlashes.ReplaceAllString(apiURL, "")

	restURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, err
	}

	wsURL, err := getWebsocketURL(restURL)
	if err != nil {
		return nil, err
	}

	client = &Client{
		transport: transport,
	}

	client.REST = &RESTClient{
		client: client,
		url:    restURL,
	}

	client.WS = &WSClient{
		commandUpdateHandler: config.CommandUpdateHandler,
		datarefUpdateHandler: config.DatarefUpdateHandler,
		client:               client,
		reqHistory:           newReqHistory(),
		resultHandler:        config.ResultHandler,
		url:                  wsURL,
	}

	return client, nil
}

func getWebsocketURL(restURL *url.URL) (*url.URL, error) {
	wsURL := *restURL
	switch restURL.Scheme {
	case "https":
		wsURL.Scheme = "wss"
	case "http":
		wsURL.Scheme = "ws"
	default:
		return nil, fmt.Errorf("invalid URL scheme: %s", restURL.Scheme)
	}
	wsURL.Path = "/api/v2"
	return &wsURL, nil
}

func (xpc *RESTClient) makeRequest(
	ctx context.Context,
	method string,
	path string,
	bodyObj any,
	target any,
) error {
	// prepare body payload
	var body io.Reader
	if bodyObj != nil {
		bodyData, err := json.Marshal(bodyObj)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewBuffer(bodyData)
	}

	apiURL := xpc.url
	apiURL.Path = path

	// perform request
	request, err := http.NewRequestWithContext(ctx, method, apiURL.String(), body)
	if err != nil {
		return fmt.Errorf("failed to create new request: %w", err)
	}

	request.Header.Add("Accept", "application/json")
	if body != nil {
		request.Header.Add("Content-Type", "application/json")
	}

	client := &http.Client{Transport: xpc.client.transport}

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

func (c *Client) LoadCache(ctx context.Context) error {
	if err := c.loadCommands(ctx); err != nil {
		return err
	}
	if err := c.loadDatarefs(ctx); err != nil {
		return err
	}
	return nil
}
