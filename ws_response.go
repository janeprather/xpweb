package xpweb

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"sync"
)

// maxReqHistory sets a limit on WSReq objects stored in a reqHistory object.
// Ideally the simulator sends timely results for every request and we never climb up to this value,
// but this exists to prevent the app from exhausting memory if the simulator decides not to send
// a result for some requests.
const maxReqHistory = 1000

// wsMessageStub is a generic struct which receives inbound websocket messages.  It sets ReqID and
// Type, and remarshals the entire JSON object so that it can be fully unmarshalled into a more
// specific message struct.
type wsMessageStub struct {
	Type string `json:"type"`
	json []byte
}

func (m *wsMessageStub) UnmarshalJSON(data []byte) error {
	genericObj := make(map[string]any)
	err := json.Unmarshal(data, &genericObj)
	if err != nil {
		return err
	}
	reqType, ok := genericObj["type"]
	if !ok {
		return errors.New("JSON data does not contain type key")
	}
	m.Type, ok = reqType.(string)
	if !ok {
		return errors.New("JSON type value is not string")
	}
	m.json = data

	return nil
}

// copyTo unmarshals the message stub's JSON onto the target object
func (m wsMessageStub) copyTo(target any) error {
	return json.Unmarshal(m.json, &target)
}

// toMessage returns the complete message object for this message
func (m wsMessageStub) toMessage() (msg any, err error) {
	switch m.Type {
	case MessageTypeResult:
		msg = &WSMessageResult{}
	case MessageTypeDatarefUpdate:
		msg = &WSMessageDatarefUpdate{}
	case MessageTypeCommandUpdate:
		msg = &WSMessageCommandUpdate{}
	default:
		return nil, fmt.Errorf("unknown message type: %s", m.Type)
	}
	if err = m.copyTo(msg); err != nil {
		return nil, err
	}
	return msg, nil
}

type WSMessageResult struct {
	ReqID        uint64 `json:"req_id"`
	Type         string `json:"type"`
	Success      bool   `json:"success"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	Req          *WSReq `json:"-"`
}

func (m WSMessageResult) GetType() string { return m.Type }

type WSDatarefValuesMap map[uint64]*DatarefValue

func (m *WSDatarefValuesMap) UnmarshalJSON(data []byte) error {
	// inbound data has dataref IDs as strings for JSON object keys
	*m = make(WSDatarefValuesMap)
	valMap := *m
	dataMap := make(map[string]any)
	if err := json.Unmarshal(data, &dataMap); err != nil {
		return err
	}
	for idString, val := range dataMap {
		id, err := strconv.ParseUint(idString, 10, 64)
		if err != nil {
			return err
		}
		valMap[id] = &DatarefValue{Value: val}
	}
	return nil
}

type WSMessageDatarefUpdate struct {
	Type string             `json:"type"`
	Data WSDatarefValuesMap `json:"data"`
}

func (m WSMessageDatarefUpdate) GetType() string { return m.Type }

// populateDatarefs uses the cache from a specified WSClient to populate the Datarefs into the
// DatarefValues objects.  This is expected to be called by the WSClient's message reading/handling
// loop/routine.
func (u *WSMessageDatarefUpdate) populateDatarefs(wsc *WSClient) {
	for drefID, drefValue := range u.Data {
		drefValue.Dataref = wsc.client.GetDatarefByID(drefID)
	}
}

// CommandStatus contains the active status of a Command.
type CommandStatus struct {
	Command  *Command
	IsActive bool
}

// WSCommandStatusMap is a structure of the data included in a command_update_is_active message
// from the websocket service.
type WSCommandStatusMap map[uint64]*CommandStatus

// UnmarshalJSON handles converting data from the JSON data into the desired structure.
func (m *WSCommandStatusMap) UnmarshalJSON(data []byte) error {
	// inbound data has command IDs as strings for JSON object keys
	*m = make(WSCommandStatusMap)
	valMap := *m
	dataMap := make(map[string]bool)
	if err := json.Unmarshal(data, &dataMap); err != nil {
		return err
	}
	for idString, isActive := range dataMap {
		id, err := strconv.ParseUint(idString, 10, 64)
		if err != nil {
			return err
		}
		valMap[id] = &CommandStatus{IsActive: isActive}
	}
	return nil
}

// WSMessageCommandUpdate is the structure of a command_update_is_active message from the
// websocket service.
type WSMessageCommandUpdate struct {
	Type string `json:"type"`
	Data WSCommandStatusMap
}

func (m WSMessageCommandUpdate) GetType() string { return m.Type }

// populateCommands uses the cache from a specified WSClient to populate the Commands into the
// CommandStatus objects.  This is expected to be called by the WSClient's message reading/handling
// loop/routine.
func (u *WSMessageCommandUpdate) populateCommands(wsc *WSClient) {
	for cmdID, cmdStatus := range u.Data {
		cmdStatus.Command = wsc.client.GetCommandByID(cmdID)
	}
}

// CommandUpdateHandler is a function which performs some action for any incoming
// [WSMessageCommandUpdate] sent by the websocket service.
type CommandUpdateHandler func(*WSMessageCommandUpdate)

// DatarefUpdateHandler is a function which performs some action for any incoming
// [WSMessageDatarefUpdate] sent by the websocket service.
type DatarefUpdateHandler func(*WSMessageDatarefUpdate)

// ResultHandler is a function which performs some action for a given WSMessageResult sent
// back from the websocket service.
type ResultHandler func(*WSMessageResult)

// reqHistory is a means to store submitted requests so they can be looked up when a result is
// received.
type reqHistory struct {
	requests map[uint64]*WSReq
	lock     sync.RWMutex
}

func newReqHistory() *reqHistory {
	return &reqHistory{requests: make(map[uint64]*WSReq)}
}

func (rh *reqHistory) add(req *WSReq) {
	rh.lock.Lock()
	defer rh.lock.Unlock()
	rh.requests[req.ReqID] = req

	// trim handlers map down to limit if it has been exceeded
	requestCount := len(rh.requests)
	if requestCount > maxReqHistory {
		numToTrim := requestCount - maxReqHistory
		reqIDs := slices.Collect(maps.Keys(rh.requests))
		slices.Sort(reqIDs)
		for _, removeID := range reqIDs[0:numToTrim] {
			delete(rh.requests, removeID)
		}
	}
}

func (rh *reqHistory) get(reqID uint64) *WSReq {
	rh.lock.RLock()
	defer rh.lock.RUnlock()
	return rh.requests[reqID]
}

func (rh *reqHistory) delete(reqID uint64) {
	rh.lock.Lock()
	defer rh.lock.Unlock()
	delete(rh.requests, reqID)
}

func (rh *reqHistory) applyToResult(msg *WSMessageResult) {
	req := rh.get(msg.ReqID)
	if req != nil {
		rh.delete(msg.ReqID)
		msg.Req = req
	}
}
