package xpweb

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
)

type datarefsResponse struct {
	Data []*Dataref `json:"data"`
}

// Dataref is a definition of a dataref provided by the simulator.
type Dataref struct {
	// The ID of the dataref.  This may change between simulator sessions, but will remain static
	// within any given session, including across aircraft loads and unloads.
	ID uint64 `json:"id"`
	// The fully qualified name of the dataref, as used by the simulator and plugins.
	Name string `json:"name"`
	// The type of the dataref value(s).
	ValueType ValueType `json:"value_type"`
}

type datarefsCountResponse struct {
	Data int `json:"data"`
}

type datarefValueResponse struct {
	Data any `json:"data"`
}

type datarefValuePatch struct {
	Data any `json:"data"`
}

// ValueType is a string representing a dataref value type which may be provided by the simulator.
type ValueType string

const (
	ValueTypeFloat      ValueType = "float"
	ValueTypeDouble     ValueType = "double"
	ValueTypeInt        ValueType = "int"
	ValueTypeIntArray   ValueType = "int_array"
	ValueTypeFloatArray ValueType = "float_array"
	ValueTypeData       ValueType = "data"
)

// DatarefValue is a type-agnostic object containing a dataref value.  The ValueType attribute may
// be checked if necessary, and an appropriate method may be called to return the typed value.
//   - float - DatarefValue.GetFloatValue
//   - double - DatarefValue.GetFloatValue
//   - int - DatarefValue.GetIntValue
//   - int_array - DatarefValue.GetIntArrayValue
//   - float_array - DatarefValue.GetFloatArrayValue
//   - data - DatarefValue.GetByteArrayValue or DatarefValue.GetStringValue
type DatarefValue struct {
	Dataref *Dataref
	Value   any
}

// GetFloatValue returns a float32 dataref value.
func (v *DatarefValue) GetFloatValue() float64 {
	if v != nil {
		if x, ok := v.Value.(float64); ok {
			return x
		}
	}
	return 0
}

// GetIntValue returns an int dataref value.
func (v *DatarefValue) GetIntValue() int {
	if v != nil {
		if x, ok := v.Value.(float64); ok {
			return int(x)
		}
	}
	return 0
}

// GetIntArrayValue returns an int slice dataref value.
func (v *DatarefValue) GetIntArrayValue() []int {
	if v != nil {
		if x, ok := v.Value.([]any); ok {
			var val []int
			for _, itemV := range x {
				if item, ok := itemV.(float64); ok {
					val = append(val, int(item))
				} else {
					// non-numeric value, bogus data
					return nil
				}
			}
			return val
		}
	}
	return nil
}

// GetFloatArrayValue returns a float slice dataref value.
func (v *DatarefValue) GetFloatArrayValue() []float64 {
	if v != nil {
		if x, ok := v.Value.([]any); ok {
			var val []float64
			for _, itemV := range x {
				if item, ok := itemV.(float64); ok {
					val = append(val, item)
				} else {
					// non-numeric value, bogus data
					return nil
				}
			}
			return val
		}
	}
	return nil
}

// GetByteArrayValue returns a byte slice representation of a data dataref value.
func (v *DatarefValue) GetByteArrayValue() []byte {
	if v != nil {
		if x, ok := v.Value.(string); ok {
			decodedBytes, err := base64.StdEncoding.DecodeString(x)
			if err != nil {
				return nil
			}
			return decodedBytes
		}
	}
	return nil
}

// GetStringValue returns a string representation of a data dataref value.
func (v *DatarefValue) GetStringValue() string {
	return string(v.GetByteArrayValue())
}

// GetDatarefs fetches and returns a list of available datarefs from the simulator.
func (c *RESTClient) GetDatarefs(ctx context.Context) ([]*Dataref, error) {
	datarefsResp := &datarefsResponse{}
	err := c.makeRequest(ctx, http.MethodGet, "/api/v2/datarefs", nil, datarefsResp)
	if err != nil {
		return nil, err
	}
	return datarefsResp.Data, nil
}

// GetDatarefsCount returns the number of total datarefs available.
func (c *RESTClient) GetDatarefsCount(ctx context.Context) (int, error) {
	datarefsCountResp := &datarefsCountResponse{}
	err := c.makeRequest(ctx, http.MethodGet, "/api/v2/datarefs/count", nil, datarefsCountResp)
	if err != nil {
		return 0, err
	}
	return datarefsCountResp.Data, nil
}

// GetDatarefByID returns the [Dataref] object with the specified ID.  If no such dataref is
// cached, a value of nil will be returned.
func (c *Client) GetDatarefByID(id uint64) (dref *Dataref) {
	c.datarefsLock.RLock()
	defer c.datarefsLock.RUnlock()

	if dataref, exists := c.datarefsByID[id]; exists {
		dref = dataref
	}
	return
}

// GetDatarefByName returns the [Dataref] object with the specified name.  If no such dataref is
// cached, a value of nil will be returned.
func (c *Client) GetDatarefByName(name string) (dref *Dataref) {
	c.datarefsLock.RLock()
	defer c.datarefsLock.RUnlock()

	if dataref, exists := c.datarefsByName[name]; exists {
		dref = dataref
	}
	return
}

// GetDatarefID returns the ID of the [Dataref] with the specified name.  If no such dataref
// is found, an value of zero is returned.
func (c *Client) GetDatarefID(name string) (id uint64) {
	if dref := c.GetDatarefByName(name); dref != nil {
		id = dref.ID
	}
	return
}

// GetDatarefName returns the name of the [Dataref] with the specified ID.  If no such dataref
// is found, an empty string value is returned.
func (c *Client) GetDatarefName(id uint64) (name string) {
	if dref := c.GetDatarefByID(id); dref != nil {
		name = dref.Name
	}
	return
}

// loadDatarefs should be called after the client is instantiated, to populate a cache of dataref
// ID and name mappings.
func (xpc *Client) loadDatarefs(ctx context.Context) error {
	xpc.datarefsLock.Lock()
	defer xpc.datarefsLock.Unlock()

	datarefs, err := xpc.REST.GetDatarefs(ctx)
	if err != nil {
		return err
	}

	xpc.datarefsByID = make(datarefsIDMap)
	xpc.datarefsByName = make(datarefsNameMap)

	for _, dataref := range datarefs {
		xpc.datarefsByID[dataref.ID] = dataref
		xpc.datarefsByName[dataref.Name] = dataref
	}

	return nil
}

// GetDatarefValue returns a type-agnostic DatarefValue object containing the value of the dataref
// with the specified name.
func (c *RESTClient) GetDatarefValue(ctx context.Context, name string) (*DatarefValue, error) {
	dref := c.client.GetDatarefByName(name)
	if dref == nil {
		return nil, fmt.Errorf("no such dataref: %s", name)
	}

	path := fmt.Sprintf("/api/v2/datarefs/%d/value", dref.ID)
	datarefValueResp := &datarefValueResponse{}
	err := c.makeRequest(ctx, http.MethodGet, path, nil, datarefValueResp)
	if err != nil {
		return nil, err
	}

	return &DatarefValue{
		Dataref: dref,
		Value:   datarefValueResp.Data,
	}, nil
}

// SetDatarefValue applies the specified value to the specified dataref.
func (c *RESTClient) SetDatarefValue(ctx context.Context, name string, value any) error {
	drefID := c.client.GetDatarefID(name)
	if drefID == 0 {
		return fmt.Errorf("no such dataref: %s", name)
	}

	path := fmt.Sprintf("/api/v2/datarefs/%d/value", drefID)
	payload := genSetDatarefValuePayload(value)

	err := c.makeRequest(ctx, http.MethodPatch, path, payload, nil)
	if err != nil {
		return err
	}

	return nil
}

// SetDatarefElementValue applies the specified value to the specified element index of the
// specified array type dataref.
func (c *RESTClient) SetDatarefElementValue(
	ctx context.Context,
	name string,
	index int,
	value any,
) error {
	drefID := c.client.GetDatarefID(name)
	if drefID == 0 {
		return fmt.Errorf("no such dataref: %s", name)
	}

	path := fmt.Sprintf("/api/v2/datarefs/%d/value?index=%d", drefID, index)
	payload := genSetDatarefValuePayload(value)

	err := c.makeRequest(ctx, http.MethodPatch, path, payload, nil)
	if err != nil {
		return err
	}

	return nil
}

// genSetDatarefValuePayload generates a datarefValuePatch object for a given value.
func genSetDatarefValuePayload(value any) *datarefValuePatch {
	payload := &datarefValuePatch{}

	// data types must be base64 encoded
	switch realValue := value.(type) {
	case string:
		payload.Data = base64.StdEncoding.EncodeToString([]byte(realValue))
	case []byte:
		payload.Data = base64.StdEncoding.EncodeToString(realValue)
	default:
		// numbers and arrays of numbers are sent verbatim
		payload.Data = realValue
	}
	return payload
}
