package xpweb

import (
	"context"
	"fmt"
	"net/http"
)

type commandsResponse struct {
	Data []*Command `json:"data"`
}

type commandsCountResponse struct {
	Data int `json:"data"`
}

type commandPost struct {
	Duration float64 `json:"duration"`
}

// Dataref is a definition of a command provided by the simulator.
type Command struct {
	// The ID of the command.  This may change between simulator sessions, but will remain static
	// within any given session, including across aircraft loads and unloads.
	ID uint `json:"id"`
	// The fully qualified name of the command, as used by the simulator and plugins.
	Name string `json:"name"`
	// The human readable description of what the command does.
	Description string `json:"description"`
}

// GetCommands fetches and returns a list of available commands from the simulator.
func (xpc *XPClient) GetCommands(ctx context.Context) ([]*Command, error) {
	commandsResp := &commandsResponse{}
	err := xpc.RestRequest(ctx, http.MethodGet, "/api/v2/commands", nil, commandsResp)
	if err != nil {
		return nil, err
	}
	return commandsResp.Data, nil
}

// GetCommandsCount returns the number of total commands available.
func (xpc *XPClient) GetCommandsCount(ctx context.Context) (int, error) {
	commandsCountResp := &commandsCountResponse{}
	err := xpc.RestRequest(ctx, http.MethodGet, "/api/v2/commands/count", nil, commandsCountResp)
	if err != nil {
		return 0, err
	}
	return commandsCountResp.Data, nil
}

// GetCommandByName returns the Command object with the specified name.  This only works if the
// XPClient.LoadCommands method has already been called.
func (xpc *XPClient) GetCommandByName(ctx context.Context, name string) (*Command, error) {
	xpc.commandsLock.RLock()
	defer xpc.commandsLock.RUnlock()

	command, exists := xpc.commands[name]
	if !exists {
		return nil, fmt.Errorf("no command exists with name %s", name)
	}

	return command, nil
}

// LoadCommands should be called after the client is instantiated, to populate a cache of command
// ID mappings.  Attempting to activate commands will fail if LoadCommands has not yet been called.
// It will not need to be called again unless the simulator is restarted.
func (xpc *XPClient) LoadCommands(ctx context.Context) error {
	xpc.commandsLock.Lock()
	defer xpc.commandsLock.Unlock()

	commands, err := xpc.GetCommands(ctx)
	if err != nil {
		return err
	}

	xpc.commands = make(CommandsMap)
	for _, command := range commands {
		xpc.commands[command.Name] = command
	}

	return nil
}

// ActivateCommand runs a command for a fixed duration. A zero duration will cause the command to
// be triggered on and off immediately but not be held down.  The maximum duration is 10 seconds.
func (xpc *XPClient) ActivateCommand(ctx context.Context, name string, duration float64) error {
	command, err := xpc.GetCommandByName(ctx, name)
	if err != nil {
		return fmt.Errorf("getDatarefID(): %w", err)
	}

	path := fmt.Sprintf("/api/v2/command/%d/activate", command.ID)
	payload := &commandPost{Duration: duration}

	err = xpc.RestRequest(ctx, http.MethodPost, path, payload, nil)
	if err != nil {
		return err
	}

	return nil
}
