package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	webxp "github.com/janeprather/xpweb"
)

var url string

func init() {
	flag.StringVar(&url, "url", "", "the URL to target, if not the default")
	flag.Parse()
}

func main() {
	ctx := context.Background()
	client := &webxp.XPClient{URL: url}
	capabilities, err := client.GetCapabilities(ctx)
	if err != nil {
		panic(fmt.Sprintf("GetCapabilities(): %s\n", err.Error()))
	}
	fmt.Printf("Capabilities\n  API Versions: %s\n  X-Plane Version: %s\n\n",
		strings.Join(capabilities.API.Versions, ", "), capabilities.XPlane.Version)

	if err := client.LoadDatarefs(ctx); err != nil {
		panic(fmt.Sprintf("LoadDatarefs(): %s\n", err.Error()))
	}

	datarefsCount, err := client.GetDatarefsCount(ctx)
	if err != nil {
		panic(fmt.Sprintf("GetDatarefsCount(): %s\n", err.Error()))
	}
	fmt.Printf("Datarefs Count: %d\n\n", datarefsCount)

	acfNameVal, err := client.GetDatarefValue(ctx, "sim/aircraft/view/acf_ui_name")
	if err != nil {
		panic(fmt.Sprintf("GetDatarefValue(): %s\n", err.Error()))
	}
	if acfNameVal.ValueType == webxp.ValueTypeData {
		fmt.Printf("Loaded Aircraft: %s\n\n", acfNameVal.GetStringValue())
	} else {
		fmt.Printf("unexpected type for acf name: %s\n", acfNameVal.ValueType)
	}

	if err := halveFuel(ctx, client); err != nil {
		panic(fmt.Sprintf("halveFuel(): %s\n", err.Error()))
	}
}

func halveFuel(ctx context.Context, client *webxp.XPClient) error {
	fuelVal, err := client.GetDatarefValue(ctx, "sim/flightmodel/weight/m_fuel")
	if err != nil {
		return fmt.Errorf("GetDatarefValue(): %w", err)
	}

	fuel := fuelVal.GetFloatArrayValue()

	for idx, tankFuel := range fuel {
		fuel[idx] = tankFuel / 2
	}

	err = client.SetDatarefValue(ctx, "sim/flightmodel/weight/m_fuel", fuel)
	if err != nil {
		return fmt.Errorf("SetDatarefValue(): %w", err)
	}

	return nil
}
