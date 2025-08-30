package xpweb

import (
	"context"
	"fmt"
	"net/http"
)

type Capabilities struct {
	API struct {
		Versions []string `json:"versions"`
	} `json:"api"`
	XPlane struct {
		Version string `json:"version"`
	} `json:"x-plane"`
}

func (xpc *XPClient) GetCapabilities(ctx context.Context) (*Capabilities, error) {
	capabilities := &Capabilities{}
	err := xpc.RestRequest(ctx, http.MethodGet, "/api/capabilities", nil, capabilities)
	if err != nil {
		return nil, fmt.Errorf("REST request failed: %w", err)
	}
	return capabilities, nil
}
