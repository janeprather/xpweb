package xpweb

import (
	"context"
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

func (c *RESTClient) GetCapabilities(ctx context.Context) (*Capabilities, error) {
	capabilities := &Capabilities{}
	err := c.makeRequest(ctx, http.MethodGet, "/api/capabilities", nil, capabilities)
	if err != nil {
		return nil, err
	}
	return capabilities, nil
}
