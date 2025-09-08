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
	ID uint64 `json:"id"`
	// The fully qualified name of the command, as used by the simulator and plugins.
	Name string `json:"name"`
	// The human readable description of what the command does.
	Description string `json:"description"`
}

// GetCommands fetches and returns a list of available commands from the simulator.
func (c *RESTClient) GetCommands(ctx context.Context) ([]*Command, error) {
	commandsResp := &commandsResponse{}
	err := c.makeRequest(ctx, http.MethodGet, "/api/v2/commands", nil, commandsResp)
	if err != nil {
		return nil, err
	}
	return commandsResp.Data, nil
}

// GetCommandsCount returns the number of total commands available.
func (c *RESTClient) GetCommandsCount(ctx context.Context) (int, error) {
	commandsCountResp := &commandsCountResponse{}
	err := c.makeRequest(ctx, http.MethodGet, "/api/v2/commands/count", nil, commandsCountResp)
	if err != nil {
		return 0, err
	}
	return commandsCountResp.Data, nil
}

// GetCommandByID returns the [Command] object with the specified ID value.  If no such command
// is cached, a value of nil will be returned.
func (c *Client) GetCommandByID(id uint64) (cmd *Command) {
	c.commandsLock.RLock()
	defer c.commandsLock.RUnlock()

	if command, exists := c.commandsByID[id]; exists {
		cmd = command
	}
	return
}

// GetCommandByName returns the [Command] object with the specified name.  If no such command
// is cached, a value of nil will be returned.
func (c *Client) GetCommandByName(name string) (cmd *Command) {
	c.commandsLock.RLock()
	defer c.commandsLock.RUnlock()

	if command, exists := c.commandsByName[name]; exists {
		cmd = command
	}
	return
}

// GetCommandID returns the ID of the [Command] with the specified name.  If no such command
// is found, a value of zero is returned.
func (c *Client) GetCommandID(name string) (id uint64) {
	if cmd := c.GetCommandByName(name); cmd != nil {
		id = cmd.ID
	}
	return
}

// GetCommandName returns the name of the [Command] with the specified ID.  If no such command
// is found, an empty string value is returned.
func (c *Client) GetCommandName(id uint64) (name string) {
	if cmd := c.GetCommandByID(id); cmd != nil {
		name = cmd.Name
	}
	return
}

// loadCommands should be called after the client is instantiated, to populate a cache of command
// ID mappings.
func (c *Client) loadCommands(ctx context.Context) error {
	c.commandsLock.Lock()
	defer c.commandsLock.Unlock()

	commands, err := c.REST.GetCommands(ctx)
	if err != nil {
		return err
	}

	c.commandsByID = make(commandsIDMap)
	c.commandsByName = make(commandsNameMap)

	for _, command := range commands {
		c.commandsByID[command.ID] = command
		c.commandsByName[command.Name] = command
	}

	return nil
}

// ActivateCommand runs a command for a fixed duration. A zero duration will cause the command to
// be triggered on and off immediately but not be held down.  The maximum duration is 10 seconds.
func (c *RESTClient) ActivateCommand(ctx context.Context, name string, duration float64) error {
	command := c.client.GetCommandByName(name)
	if command == nil {
		return fmt.Errorf("no such command: %s", name)
	}

	path := fmt.Sprintf("/api/v2/command/%d/activate", command.ID)
	payload := &commandPost{Duration: duration}

	err := c.makeRequest(ctx, http.MethodPost, path, payload, nil)
	if err != nil {
		return err
	}

	return nil
}
