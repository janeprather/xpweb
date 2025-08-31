package xpweb

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	// Don't actually run the example because it requires an active connection to X-Plane 12.
	// TODO: mock up an http.RoundTripper that allows us to simulate legit 200 ok and other
	// responses from the API for tests and examples.
}

// Start the Skyhawk G1000 from cold and dark.
func ExampleXPClient_ActivateCommand() {
	ctx := context.Background()
	client := &XPClient{}

	if err := client.LoadCommands(ctx); err != nil {
		panic(err)
	}

	// turn on battery
	if err := client.ActivateCommand(ctx, "sim/electrical/battery_1_on", 0); err != nil {
		panic(err)
	}

	// turn on alternator
	if err := client.ActivateCommand(ctx, "sim/electrical/generator_1_on", 0); err != nil {
		panic(err)
	}

	// turn on beacon
	if err := client.ActivateCommand(ctx, "sim/lights/beacon_lights_on", 0); err != nil {
		panic(err)
	}
	time.Sleep(time.Second)

	// set mixture to full rich
	fmt.Println("Setting mixture to max rich")
	if err := client.ActivateCommand(ctx, "sim/engines/mixture_max", 0); err != nil {
		panic(err)
	}

	time.Sleep(time.Second)

	// set fuel selector to both tanks
	if err := client.ActivateCommand(ctx, "sim/fuel/fuel_selector_all", 0); err != nil {
		panic(err)
	}
	time.Sleep(time.Second)

	// set both magnetos
	if err := client.ActivateCommand(ctx, "sim/magnetos/magnetos_both", 0); err != nil {
		panic(err)
	}

	// engage starter for 2 seconds
	if err := client.ActivateCommand(ctx, "sim/engines/engage_starters", 2); err != nil {
		panic(err)
	}
	time.Sleep(2100 * time.Millisecond)

	// turn on avionics (MFD)
	if err := client.ActivateCommand(ctx, "sim/systems/avionics_on", 0); err != nil {
		panic(err)
	}

	// wait for MFD to boot up
	time.Sleep(6 * time.Second)

	// press ENT on the MFD
	if err := client.ActivateCommand(ctx, "sim/GPS/g1000n3_ent", 0); err != nil {
		panic(err)
	}
}
