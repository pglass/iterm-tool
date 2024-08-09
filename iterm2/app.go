package iterm2

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/pglass/iterm-tool/iterm2/api"
	"github.com/pglass/iterm-tool/iterm2/client"
)

// App represents an open iTerm2 application
type App interface {
	io.Closer

	CreateWindow(*CreateWindowOpts) (Window, error)
	ListWindows() ([]Window, error)
	SelectMenuItem(item string) error
	Activate(raiseAllWindows, ignoreOtherApps bool) error
}

type CreateWindowOpts struct {
	CustomProfileProperties CustomProfileProperties
}

type CustomProfileProperties struct {
	TitleComponents TitleComponent
}

func (c CustomProfileProperties) toProperties() ([]*api.ProfileProperty, error) {
	result := []*api.ProfileProperty{}
	if c.TitleComponents != 0 {
		data, err := json.Marshal(c.TitleComponents)
		if err != nil {
			return nil, err
		}
		result = append(result, &api.ProfileProperty{
			Key:       str("Title Components"),
			JsonValue: str(string(data)),
		})
	}
	return result, nil
}

type TitleComponent int

// https://github.com/gnachman/iTerm2/blob/1386b4fd41e18f55a25273aa4875fb604865afe9/api/library/python/iterm2/iterm2/profile.py#L108
const (
	TitleComponentSessionName           = (1 << 0)
	TitleComponentJob                   = (1 << 1)
	TitleComponentWorkingDirectory      = (1 << 2)
	TitleComponentTTY                   = (1 << 3)
	TitleComponentCustom                = (1 << 4) // Mutually exclusive with all other options.
	TitleComponentProfileName           = (1 << 5)
	TitleComponentProfileAndSessionName = (1 << 6)
	TitleComponentUser                  = (1 << 7)
	TitleComponentHost                  = (1 << 8)
	TitleComponentCommandLine           = (1 << 9)
	TitleComponentSize                  = (1 << 10)
)

// NewApp establishes a connection
// with iTerm2 and returns an App.
// Name is an optional parameter that
// can be used to register your application
// name with iTerm2 so that it doesn't
// require explicit permissions every
// time you run the plugin.
func NewApp(name string) (App, error) {
	c, err := client.New(name)
	if err != nil {
		return nil, err
	}

	return &app{c: c}, nil
}

type app struct {
	c *client.Client
}

func (a *app) Activate(raiseAllWindows bool, ignoreOtherApps bool) error {
	_, err := a.c.Call(&api.ClientOriginatedMessage{
		Submessage: &api.ClientOriginatedMessage_ActivateRequest{ActivateRequest: &api.ActivateRequest{
			OrderWindowFront: b(true),
			ActivateApp: &api.ActivateRequest_App{
				RaiseAllWindows:   &raiseAllWindows,
				IgnoringOtherApps: &ignoreOtherApps,
			},
		}},
	})
	return err
}

func (a *app) CreateWindow(opts *CreateWindowOpts) (Window, error) {
	var customProps []*api.ProfileProperty
	if opts != nil {
		props, err := opts.CustomProfileProperties.toProperties()
		if err != nil {
			return nil, err
		}
		customProps = props
	}

	resp, err := a.c.Call(&api.ClientOriginatedMessage{
		Submessage: &api.ClientOriginatedMessage_CreateTabRequest{
			CreateTabRequest: &api.CreateTabRequest{
				CustomProfileProperties: customProps,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not create window tab: %w", err)
	}
	ctr := resp.GetCreateTabResponse()
	if ctr.GetStatus() != api.CreateTabResponse_OK {
		return nil, fmt.Errorf("unexpected window tab status: %s", ctr.GetStatus())
	}
	return &window{
		c:       a.c,
		id:      ctr.GetWindowId(),
		session: ctr.GetSessionId(),
	}, nil
}

func (a *app) ListWindows() ([]Window, error) {
	list := []Window{}
	resp, err := a.c.Call(&api.ClientOriginatedMessage{
		Submessage: &api.ClientOriginatedMessage_ListSessionsRequest{
			ListSessionsRequest: &api.ListSessionsRequest{},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not list sessions: %w", err)
	}
	for _, w := range resp.GetListSessionsResponse().GetWindows() {
		list = append(list, &window{
			c:  a.c,
			id: w.GetWindowId(),
		})
	}
	return list, nil
}

func (a *app) Close() error {
	return a.c.Close()
}

func str(s string) *string {
	return &s
}

func b(b bool) *bool {
	return &b
}

func (a *app) SelectMenuItem(item string) error {
	resp, err := a.c.Call(&api.ClientOriginatedMessage{
		Submessage: &api.ClientOriginatedMessage_MenuItemRequest{
			MenuItemRequest: &api.MenuItemRequest{
				Identifier: &item,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("error selecting menu item %q: %w", item, err)
	}
	if resp.GetMenuItemResponse().GetStatus() != api.MenuItemResponse_OK {
		return fmt.Errorf("menu item %q returned unexpected status: %q", item, resp.GetMenuItemResponse().GetStatus().String())
	}
	return nil
}
