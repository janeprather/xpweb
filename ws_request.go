package xpweb

// WSReq is an object containing the payload of a websocket request.  A WSReq object is easiest to
// instantiate using the function appropriate for the type of request being made.
//
//   - [WSClient.NewWSReqCommandSetIsActive] (command_set_is_active)
//   - [WSClient.NewWSReqDatarefSubscribe] (dataref_subscribe_values)
//   - [WSClient.NewWSReqDatarefUnsubscribe] (dataref_unsubscribe_values, specified datarefs)
//   - [WSClient.NewWSReqDatarefUnsubscribeAll] (dataref_unsubscribe_values, all datarefs)
type WSReq struct {
	ReqID    uint64 `json:"req_id"`
	Type     string `json:"type"`
	Params   any    `json:"params"`
	wsClient *WSClient
}

// NewReq instantiates a new websocket request object having the next available request ID.  Type
// and params are not set, and an appropriate method should be called to apply them.
//
//   - [WSReq.CommandSetIsActive] for command_set_is_active
//   - [WSReq.DatarefSubscribe] for dataref_subscribe_values
//   - [WSReq.DatarefUnsubscribe] for dataref_unsubscribe_values (specified datarefs)
//   - [WSReq.DatarefUnsubscribeAll] for dataref_unsubscribe_values (all datarefs)
//
// For example:
//
//	xpWS.NewReq().CommandSetIsActive(
//		xpWS.NewCommand("sim/electrical/battery_1_on", true).WithDuration(0),
//	)
func (wsc *WSClient) NewReq() *WSReq {
	return &WSReq{ReqID: wsc.messageID.Add(1), wsClient: wsc}
}

// CommandSetIsActive applies a type of command_set_is_active and appropriate params to the
// WSReq object.  It returns a pointer to the WSReq object so that it can be chained with WSReq
// instantiation.  Pointers to one or more [WSCommand] objects should be passed as args.
func (r *WSReq) CommandSetIsActive(cmds ...*WSCommand) *WSReq {
	r.Type = MessageTypeCommandSetIsActive
	r.Params = map[string]any{"commands": cmds}
	return r
}

// CommandSubscribe applies a type of command_subscribe_is_active and appropriate params to the
// WSReq object.  It returns a pointer to the WSReq object so that it can be chained with WSReq
// instantiation.  Command name values should be passed as args.
func (r *WSReq) CommandSubscribe(cmdNames ...string) *WSReq {
	r.Type = MessageTypeCommandSub
	var cmds []map[string]uint64
	for _, cmdName := range cmdNames {
		cmdID := r.wsClient.client.GetCommandID(cmdName)
		cmds = append(cmds, map[string]uint64{"id": cmdID})
	}
	r.Params = map[string]any{"commands": cmds}
	return r
}

// CommandUnsubscribe applies a type of command_unsubscribe_is_active and appropriate params to the
// WSReq object.  It returns a pointer to the WSReq object so that it can be chained with WSReq
// instantiation.  Command name values should be passed as args.
func (r *WSReq) CommandUnsubscribe(cmdNames ...string) *WSReq {
	r.Type = MessageTypeCommandUnsub
	var cmds []map[string]uint64
	for _, cmdName := range cmdNames {
		cmdID := r.wsClient.client.GetCommandID(cmdName)
		cmds = append(cmds, map[string]uint64{"id": cmdID})
	}
	r.Params = map[string]any{"commands": cmds}
	return r
}

// DatarefUnsubscribeAll applies a type of command_unsubscribe_is_active and a params value which
// will unsubscribe from all currently subscribed datarefs.  It returns a pointer to the WSReq
// object so that it ican be chained with WSReq instantiation.
func (r *WSReq) CommandUnsubscribeAll() *WSReq {
	r.Type = MessageTypeCommandUnsub
	r.Params = map[string]any{"commands": "all"}
	return r
}

// DatarefSubscribe applies a type of dataref_subscribe_values and appropriate params to the WSReq
// object.  It returns a pointer to the WSReq object so that it can be chained with WSReq
// instantiation.  Pointers to one or more [WSDataref] objects should be passed as args.
func (r *WSReq) DatarefSubscribe(datarefs ...*WSDataref) *WSReq {
	r.Type = MessageTypeDatarefSub
	r.Params = map[string]any{"datarefs": datarefs}
	return r
}

// DatarefUnsubscribe applies a type of dataref_unsubscribe_values and appropriate params to the
// WSReq object.  It returns a pointer to the WSReq object so that it can be chained with WSReq
// instantiation.  Pointers to one or more [WSDataref] objects should be passed as args.
func (r *WSReq) DatarefUnsubscribe(datarefs ...*WSDataref) *WSReq {
	r.Type = MessageTypeDatarefUnsub
	r.Params = map[string]any{"datarefs": datarefs}
	return r
}

// DatarefUnsubscribeAll applies a type of dataref_unsubscribe_values and a params value which will
// unsubscribe from all currently subscribed datarefs.  It returns a pointer to the WSReq object so
// that it ican be chained with WSReq instantiation.
func (r *WSReq) DatarefUnsubscribeAll() *WSReq {
	r.Type = MessageTypeDatarefUnsub
	r.Params = map[string]any{"datarefs": "all"}
	return r
}

// DatarefSet applies a type of dataref_set_values and appropriate params to the WSReq object.  It
// returns a pointer to the WSReq object so that it can be chained with WSReq instantiation.
// Pointers to one or more [WSDatarefValue] objects should be passed as args.
func (r *WSReq) DatarefSet(datarefs ...*WSDatarefValue) *WSReq {
	r.Type = MessageTypeDatarefSet
	r.Params = map[string]any{"datarefs": datarefs}
	return r
}

// Send submits the WSReq object to the websocket service.
func (r *WSReq) Send() error {
	return r.wsClient.Send(r)
}

// WSCommand is a structure which is included in websocket requests to set whether a command is
// active.  It is easiest to instantiate a WSCommand object using [WithCommand] or
// [Client.WithCommand].
type WSCommand struct {
	ID       uint64   `json:"id"`
	IsActive bool     `json:"is_active"`
	Duration *float64 `json:"duration,omitempty"`
}

// WithDuration applies a duration to the WSCommand object.  It returns a pointer to the WSCommand
// object so that it can be chained with WSCommand instantiation.  A value of zero makes it an
// immediate toggle (e.g. pressing and releasing a button right away).  A positive value sets the
// number of seconds to wait between reverting the active value.
func (c *WSCommand) WithDuration(duration float64) *WSCommand {
	c.Duration = ptr(duration)
	return c
}

// NewWSCommand returns a WSCommand which can be passed to be passed to
// [NewWSReqCommandSetIsActive], and which will apply the command for an indefinite period.
// To perform an instant toggle of the command (e.g. a button press) or set a duration (e.g.
// pressing a starter button), use the [WithCommand.Timed] method on the returned value.
//
// Indefinite:
//
//	WithCommand(id, true)
//
// Instant:
//
//	WithCommand(id, true).WithDuration(0)
//
// For a set number of seconds:
//
//	WithCommand(id, true).WithDuration(2.5)
func NewWSCommand(id uint64, isActive bool) *WSCommand {
	return &WSCommand{ID: id, IsActive: isActive}
}

// NewCommand behaves like [NewWSCommand] except that it takes a command name as an argument and
// uses the [Client] object's loaded command cache to map the command name to its ID value. If
// the command does not exist, an ID value of 0 will be used and a websocket request containing the
// returned value should fail.
func (wsc *WSClient) NewCommand(name string, isActive bool) *WSCommand {
	return NewWSCommand(wsc.client.GetCommandID(name), isActive)
}

// WSDataref is a structure which is included in a websocket requests to sub/unsub datarefs.  It is
// easiest to instantiate a WSDataref object using WithDataref() or WithDatarefIndex().
type WSDataref struct {
	ID    uint64 `json:"id"`
	Index any    `json:"index,omitempty"`
}

// WithIndex applies the specified single index to the WSDataref object.  It returns a pointer to
// the WSDataref so that it can be chained with WSDataref instantiation.
func (d *WSDataref) WithIndex(index int) *WSDataref {
	d.Index = index
	return d
}

// WithIndexArray applies the specified slice of index values to the WSDataref object.  It returns
// a pointer to the WSDataref so that it can be chained with WSDataref instantiation.
func (d *WSDataref) WithIndexArray(indexes []int) *WSDataref {
	d.Index = indexes
	return d
}

// NewWSDataref returns a pointer to a WSDataref object with the specified dataref ID value.
func NewWSDataref(id uint64) *WSDataref {
	return &WSDataref{ID: id}
}

// NewDataref behaves like [NewWSDataref] except that it takes a dataref name as the argument and
// uses the [Client] object's loaded dataref cache to map the dataref name to its ID value.  If
// the dataref does not exist, an ID value of 0 will be used and a websocket request containing
// the returned value should fail.
func (wsc *WSClient) NewDataref(name string) *WSDataref {
	return NewWSDataref(wsc.client.GetDatarefID(name))
}

// WSDataref is a structure which is included in a websocket requests to sub/unsub datarefs.  It is
// easiest to instantiate a WSDataref object using WithDataref() or WithDatarefIndex().
type WSDatarefValue struct {
	ID    uint64 `json:"id"`
	Value any    `json:"value"`
	Index *int   `json:"index,omitempty"`
}

// WithIndex applies the specified single index to the WSDataref object.  It returns a pointer to
// the WSDataref so that it can be chained with WSDataref instantiation.
func (d *WSDatarefValue) WithIndex(index int) *WSDatarefValue {
	d.Index = ptr(index)
	return d
}

// WithDataref returns a pointer to a WSDataref object with the specified dataref ID value.
func NewWSDatarefValue(id uint64, value any) *WSDatarefValue {
	return &WSDatarefValue{ID: id, Value: value}
}

// NewWSDatarefValue behaves like [NewWSDatarefValue] except that it takes a dataref name as the
// argument and uses the [Client] object's loaded dataref cache to map the dataref name to its ID
// value.  If the dataref does not exist, an ID value of 0 will be used and a websocket request
// containing the returned value should fail.
func (wsc *WSClient) NewDatarefValue(name string, value any) *WSDatarefValue {
	return NewWSDatarefValue(wsc.client.GetDatarefID(name), value)
}
