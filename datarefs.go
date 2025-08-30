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
	ID uint `json:"id"`
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
	ValueType ValueType
	Value     any
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
func (xpc *XPClient) GetDatarefs(ctx context.Context) ([]*Dataref, error) {
	datarefsResp := &datarefsResponse{}
	err := xpc.RestRequest(ctx, http.MethodGet, "/api/v2/datarefs", nil, datarefsResp)
	if err != nil {
		return nil, fmt.Errorf("REST request failed: %w", err)
	}
	return datarefsResp.Data, nil
}

// GetDatarefsCount returns the number of total datarefs available.
func (xpc *XPClient) GetDatarefsCount(ctx context.Context) (int, error) {
	datarefsCountResp := &datarefsCountResponse{}
	err := xpc.RestRequest(ctx, http.MethodGet, "/api/v2/datarefs/count", nil, datarefsCountResp)
	if err != nil {
		return 0, fmt.Errorf("REST request failed: %w", err)
	}
	return datarefsCountResp.Data, nil
}

// GetDatarefByName returns the Dataref object with the specified name.  This only works if the
// XPClient.LoadDatarefs method has already been called.
func (xpc *XPClient) GetDatarefByName(ctx context.Context, name string) (*Dataref, error) {
	xpc.datarefsLock.RLock()
	defer xpc.datarefsLock.RUnlock()

	dataref, exists := xpc.datarefs[name]
	if !exists {
		return nil, fmt.Errorf("no dataref exists with name %s", name)
	}

	return dataref, nil
}

// LoadDatarefs should be called after the client is instantiated, to populate a cache of dataref
// ID mappings.  Attempting to lookup dataref values will fail if LoadDatarefs has not yet been
// called.  It will not need to be called again unless the simulator is restarted.
func (xpc *XPClient) LoadDatarefs(ctx context.Context) error {
	xpc.datarefsLock.Lock()
	defer xpc.datarefsLock.Unlock()

	datarefs, err := xpc.GetDatarefs(ctx)
	if err != nil {
		return fmt.Errorf("DetDatarefs(): %w", err)
	}

	xpc.datarefs = make(DatarefsMap)
	for _, dataref := range datarefs {
		xpc.datarefs[dataref.Name] = dataref
	}

	return nil
}

// GetDatarefValue returns a type-agnostic DatarefValue object containing the value of the dataref
// with the specified name.
func (xpc *XPClient) GetDatarefValue(ctx context.Context, name string) (*DatarefValue, error) {
	dataref, err := xpc.GetDatarefByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("getDatarefID(): %w", err)
	}

	path := fmt.Sprintf("/api/v2/datarefs/%d/value", dataref.ID)
	datarefValueResp := &datarefValueResponse{}
	err = xpc.RestRequest(ctx, http.MethodGet, path, nil, datarefValueResp)
	if err != nil {
		return nil, fmt.Errorf("REST request failed: %w", err)
	}

	return &DatarefValue{
		ValueType: dataref.ValueType,
		Value:     datarefValueResp.Data,
	}, nil
}

// SetDatarefValue applies the specified value to the specified dataref.
func (xpc *XPClient) SetDatarefValue(ctx context.Context, name string, value any) error {
	dataref, err := xpc.GetDatarefByName(ctx, name)
	if err != nil {
		return fmt.Errorf("getDatarefID(): %w", err)
	}

	path := fmt.Sprintf("/api/v2/datarefs/%d/value", dataref.ID)
	payload := genSetDatarefValuePayload(value)

	err = xpc.RestRequest(ctx, http.MethodPatch, path, payload, nil)
	if err != nil {
		return fmt.Errorf("REST request failed: %w", err)
	}

	return nil
}

// SetDatarefElementValue applies the specified value to the specified element index of the
// specified array type dataref.
func (xpc *XPClient) SetDatarefElementValue(
	ctx context.Context,
	name string,
	index int,
	value any,
) error {
	dataref, err := xpc.GetDatarefByName(ctx, name)
	if err != nil {
		return fmt.Errorf("getDatarefID(): %w", err)
	}

	path := fmt.Sprintf("/api/v2/datarefs/%d/value?index=%d", dataref.ID, index)
	payload := genSetDatarefValuePayload(value)

	err = xpc.RestRequest(ctx, http.MethodPatch, path, payload, nil)
	if err != nil {
		return fmt.Errorf("REST request failed: %w", err)
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
