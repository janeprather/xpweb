package xpweb

import (
	"errors"
	"log"
	"net/url"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/net/websocket"
)

const reconnectFreq time.Duration = 5 * time.Second

const (
	MessageTypeResult             string = "result"
	MessageTypeDatarefSub         string = "dataref_subscribe_values"
	MessageTypeDatarefUpdate      string = "dataref_update_values"
	MessageTypeDatarefUnsub       string = "dataref_unsubscribe_values"
	MessageTypeDatarefSet         string = "dataref_set_values"
	MessageTypeCommandSub         string = "command_subscribe_is_active"
	MessageTypeCommandUnsub       string = "command_unsubscribe_is_active"
	MessageTypeCommandUpdate      string = "command_update_is_active"
	MessageTypeCommandSetIsActive string = "command_set_is_active"
)

// XPWebsocketClient provides functions and attributes related to Websocket API operations.
type WSClient struct {
	commandUpdateHandler CommandUpdateHandler
	datarefUpdateHandler DatarefUpdateHandler
	client               *Client
	conn                 *websocket.Conn
	messageID            atomic.Uint64
	reqHistory           *reqHistory
	resultHandler        ResultHandler
	url                  *url.URL
}

// readLoop continually reads from the websocket while the connection is open.  It should be called
// in a goroutine after the websocket connects.
func (wsc *WSClient) readLoop() {
	for {
		var inMsg wsMessageStub
		err := websocket.JSON.Receive(wsc.conn, &inMsg)
		if err != nil {
			if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNABORTED) {
				// connection reset or aborted, we should try to reconnect gracefully
				go wsc.reconnectLoop()
				return
			}
			log.Printf("failed to read message: %s\n", err.Error())
			continue
		}
		msg, err := inMsg.toMessage()
		if err != nil {
			log.Printf("failed to unmarshal incoming message: %s\n", err.Error())
			continue
		}

		switch realMsg := msg.(type) {
		case *WSMessageResult:
			if wsc.resultHandler != nil {
				wsc.reqHistory.applyToResult(realMsg)
				wsc.resultHandler(realMsg)
			}
		case *WSMessageDatarefUpdate:
			if wsc.datarefUpdateHandler != nil {
				// The UnmarshalJSON method didn't have access to the client cache, so contains
				// DatarefValue objects with nil Dataref pointers. Populate those Dataref values
				// here before passing the message to the handler.
				realMsg.populateDatarefs(wsc)
				wsc.datarefUpdateHandler(realMsg)
			}
		case *WSMessageCommandUpdate:
			if wsc.commandUpdateHandler != nil {
				// The UnmarshalJSON method didn't have access to the client cache, so contains
				// CommandStatus objects with nil Command pointers.  Populate these Command values
				// here before passing the message to the handler.
				realMsg.populateCommands(wsc)
				wsc.commandUpdateHandler(realMsg)
			}
		}
	}
}

// reconnectLoop continually attempts to continuously re-establish a websocket connection
func (xpc *WSClient) reconnectLoop() {
	for {
		err := xpc.Connect()
		if err == nil {
			// established connection
			return
		}
		log.Printf("failed to re-establish websocket connection: %s\n", err.Error())
		time.Sleep(reconnectFreq)
	}
}

// SendToWS marshals the specified object into JSON and sends it over the websocket connection.
func (c *WSClient) Send(req *WSReq) error {
	c.reqHistory.add(req)

	if err := websocket.JSON.Send(c.conn, req); err != nil {
		return err
	}

	return nil
}

// WSConnect establishes a websocket connection to the web API.  If an application calls this
// function, it must read from the channel returned by XPClient.Messages() to avoid a deadlock.
func (xpc *WSClient) Connect() (err error) {
	if xpc.conn != nil {
		xpc.Close()
	}
	xpc.conn, err = websocket.Dial(xpc.url.String(), "", xpc.client.REST.url.String())
	if err != nil {
		return err
	}
	go xpc.readLoop()
	return nil
}

// WSClose closes an established websocket connection.
func (xpc *WSClient) Close() {
	if xpc.conn != nil {
		xpc.conn.Close()
		xpc.conn = nil
	}
}
