package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/janeprather/xpweb"
	"github.com/janeprather/xpweb/names/command"
	"github.com/janeprather/xpweb/names/dataref"
)

var apiURL string

func init() {
	flag.StringVar(&apiURL, "url", "", "the URL to target, if not the default")
	flag.Parse()
}

func main() {
	ctx := context.Background()

	handleDatarefUpdate := func(msg *xpweb.WSMessageDatarefUpdate) {
		var drefUdates []string
		for _, val := range msg.Data {
			drefUdates = append(drefUdates, fmt.Sprintf("  %s: %v\n", val.Dataref.Name, val.Value))

		}
		fmt.Printf("dataref(s) update:\n%s", strings.Join(drefUdates, ""))
	}

	handleCommandUpdate := func(msg *xpweb.WSMessageCommandUpdate) {
		var cmdUdates []string
		for _, val := range msg.Data {
			cmdUdates = append(cmdUdates, fmt.Sprintf("  %s: %v\n", val.Command.Name, val.IsActive))

		}
		fmt.Printf("command(s) update:\n%s", strings.Join(cmdUdates, ""))
	}

	handleResult := func(msg *xpweb.WSMessageResult) {
		output := fmt.Sprintf("request %d (%s) result: %v", msg.ReqID, msg.Req.Type, msg.Success)
		if msg.ErrorMessage != "" {
			output += fmt.Sprintf(" (%s)", msg.ErrorMessage)
		}
		fmt.Println(output)
	}

	clientConfig := &xpweb.ClientConfig{
		URL:                  apiURL,
		CommandUpdateHandler: handleCommandUpdate,
		DatarefUpdateHandler: handleDatarefUpdate,
		ResultHandler:        handleResult,
	}

	client, err := xpweb.NewClient(clientConfig)
	if err != nil {
		panic(err)
	}

	xpREST := client.REST
	xpWS := client.WS

	capabilities, err := xpREST.GetCapabilities(ctx)
	if err != nil {
		panic(fmt.Sprintf("GetCapabilities(): %s\n", err.Error()))
	}
	fmt.Printf("Capabilities\n  API Versions: %s\n  X-Plane Version: %s\n\n",
		strings.Join(capabilities.API.Versions, ", "), capabilities.XPlane.Version)

	if err := client.LoadCache(ctx); err != nil {
		panicWithErr(err)
	}

	datarefsCount, err := xpREST.GetDatarefsCount(ctx)
	if err != nil {
		panic(fmt.Sprintf("GetDatarefsCount(): %s\n", err.Error()))
	}
	fmt.Printf("Datarefs Count: %d\n\n", datarefsCount)

	acfNameVal, err := xpREST.GetDatarefValue(ctx, dataref.SimAircraftView_acf_ui_name)
	if err != nil {
		panic(fmt.Sprintf("GetDatarefValue(): %s\n", err.Error()))
	}
	if acfNameVal.Dataref.ValueType == xpweb.ValueTypeData {
		fmt.Printf("Loaded Aircraft: %s\n\n", acfNameVal.GetStringValue())
	} else {
		fmt.Printf("unexpected type for acf name: %s\n", acfNameVal.Dataref.ValueType)
	}

	/*
		if err := halveFuel(ctx, client); err != nil {
			panic(fmt.Sprintf("halveFuel(): %s\n", err.Error()))
		}
	*/
	commandsCount, err := xpREST.GetCommandsCount(ctx)
	if err != nil {
		panicWithErr(err)
	}
	fmt.Printf("Commands Count: %d\n\n", commandsCount)

	/*
		if err := startSkyhawk(ctx, client); err != nil {
			panicWithErr(err)
		}
	*/
	/*
		numTanksVal, err := client.REST.GetDatarefValue(ctx, "sim/aircraft/overflow/acf_num_tanks")
		if err != nil {
			panic(err)
		}
		numTanks := numTanksVal.GetIntValue()

		fmt.Printf("Number of fuel tanks: %d\n", numTanks)

		maxFuelTotVal, err := xpREST.GetDatarefValue(ctx, "sim/aircraft/weight/acf_m_fuel_tot")
		if err != nil {
			panic(err)
		}
		maxFuelTot := maxFuelTotVal.GetFloatValue()

		fuelRatiosVal, err := xpREST.GetDatarefValue(ctx, "sim/aircraft/overflow/acf_tank_rat")
		if err != nil {
			panic(err)
		}
		fuelRatios := fuelRatiosVal.GetFloatArrayValue()

		//maxFuel := maxFuelVal.GetFloatArrayValue()
		fmt.Printf("Fuel fuel: %v\n", maxFuelTotVal.Value)
		fmt.Printf("fuel ratios: %v\n", fuelRatios)

		fuelVal, err := xpREST.GetDatarefValue(ctx, "sim/flightmodel/weight/m_fuel")
		if err != nil {
			panic(err)
		}

		fuel := fuelVal.GetFloatArrayValue()

		// fill er up
		for idx := range numTanks {
			fuel[idx] = maxFuelTot * fuelRatios[idx]
		}
	*/

	if err := xpWS.Connect(); err != nil {
		panic(err)
	}
	defer xpWS.Close()

	/*
		fillFuel := xpWS.NewReq().DatarefSet(
			xpWS.NewDatarefValue(dataref.SimFlightmodelWeight_m_fuel, fuel),
		)

		xpWS.ResultHandlers.Add(fillFuel.ReqID, func(msg *xpweb.WSMessageResult) {
			if msg.Success {
				fmt.Println("Fuel topped off.")
			} else {
				fmt.Println("failed to top off fuel")
			}
			if msg.ErrorMessage != "" {
				fmt.Println(msg.ErrorMessage)
			}
		})

		xpWS.Send(fillFuel)
	*/

	subMsg := xpWS.NewReq().CommandSubscribe(command.SimElectrical_battery_1_on)
	xpWS.Send(subMsg)

	fuelDrefID := client.GetDatarefID("sim/flightmodel/weight/m_fuel")
	if fuelDrefID == 0 {
		panic(errors.New("fuel dataref not found"))
	}

	if err := xpWS.NewReq().DatarefSubscribe(
		xpWS.NewDataref("sim/flightmodel/weight/m_fuel").WithIndexArray([]int{0, 1}),
	).Send(); err != nil {
		panic(err)
	}

	cmd := client.GetCommandByName(command.SimNone_none)
	if cmd == nil {
		panic(errors.New("couldn't find sim/none/none command"))
	}

	cmdMsg := xpWS.NewReq().CommandSetIsActive(
		xpweb.NewWSCommand(cmd.ID, true).WithDuration(0),
	)

	xpWS.Send(cmdMsg)

	cmdMsg = xpWS.NewReq().CommandSetIsActive(
		xpWS.NewCommand(command.SimElectrical_battery_1_on, true).WithDuration(0),
	)

	xpWS.Send(cmdMsg)

	time.Sleep(30 * time.Second)

	xpWS.Send(xpWS.NewReq().DatarefUnsubscribeAll())

	time.Sleep(10 * time.Minute)
}

func panicWithErr(err error) {
	panic(err.Error())
}

func startSkyhawk(ctx context.Context, client *xpweb.Client) error {
	fmt.Println("Starting skyhawk")

	fmt.Println("Turning on battery")
	if err := client.REST.ActivateCommand(ctx, command.SimElectrical_battery_1_on, 0); err != nil {
		return err
	}

	fmt.Println("Turning on alternator")
	if err := client.REST.ActivateCommand(ctx, command.SimElectrical_generator_1_on, 0); err != nil {
		return err
	}

	fmt.Println("Turning on beacon")
	if err := client.REST.ActivateCommand(ctx, "sim/lights/beacon_lights_on", 0); err != nil {
		return err
	}
	time.Sleep(time.Second)

	// set mixture to full rich
	fmt.Println("Setting mixture to max rich")
	if err := client.REST.ActivateCommand(ctx, "sim/engines/mixture_max", 0); err != nil {
		return err
	}

	time.Sleep(time.Second)

	// set fuel to both tanks
	fmt.Println("Selecting both fuel tanks")
	if err := client.REST.ActivateCommand(ctx, "sim/fuel/fuel_selector_all", 0); err != nil {
		return err
	}

	time.Sleep(time.Second)

	// set both magnetos
	fmt.Println("Selecting both magnetos")
	if err := client.REST.ActivateCommand(ctx, "sim/magnetos/magnetos_both", 0); err != nil {
		return err
	}

	// engage starter for 2 seconds
	fmt.Println("Engaging starter for 2 seconds")
	if err := client.REST.ActivateCommand(ctx, "sim/engines/engage_starters", 2); err != nil {
		return err
	}

	time.Sleep(2100 * time.Millisecond)

	fmt.Println("Turning on avionics")
	if err := client.REST.ActivateCommand(ctx, "sim/systems/avionics_on", 0); err != nil {
		return err
	}

	fmt.Println("Waiting for MFD to start")
	time.Sleep(6 * time.Second)

	fmt.Println("Pressing ENT on the MFD")
	if err := client.REST.ActivateCommand(ctx, "sim/GPS/g1000n3_ent", 0); err != nil {
		return err
	}

	return nil
}

func halveFuel(ctx context.Context, client *xpweb.Client) error {
	numTanksVal, err := client.REST.GetDatarefValue(ctx, "sim/aircraft/overflow/acf_num_tanks")
	if err != nil {
		return fmt.Errorf("GetDatarefValue(): %w", err)
	}
	numTanks := numTanksVal.GetIntValue()
	fmt.Printf("Number of tanks: %d\n", numTanks)

	fuelVal, err := client.REST.GetDatarefValue(ctx, "sim/flightmodel/weight/m_fuel")
	if err != nil {
		return fmt.Errorf("GetDatarefValue(): %w", err)
	}

	fuel := fuelVal.GetFloatArrayValue()

	for idx := range numTanks {
		fmt.Printf("Tank %d: %.3f\n", idx, fuel[idx])
	}

	for idx, tankFuel := range fuel {
		fuel[idx] = tankFuel / 2
	}

	err = client.REST.SetDatarefValue(ctx, "sim/flightmodel/weight/m_fuel", fuel)
	if err != nil {
		return fmt.Errorf("SetDatarefValue(): %w", err)
	}

	return nil
}
