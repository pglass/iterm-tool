package iterm2

import (
	"fmt"

	"github.com/pglass/iterm-tool/iterm2/api"
	"github.com/pglass/iterm-tool/iterm2/client"
)

// Session represents an iTerm2 Session which is a pane
// within a Tab where the terminal is active
type Session interface {
	SendText(s string) error
	Activate(selectTab, orderWindowFront bool) error
	SplitPane(opts SplitPaneOptions) (Session, error)
	GetSessionID() string
	SetName(string) error
	GetVariable(string) (string, error)
}

// SplitPaneOptions for customizing the new pane session.
// More options can be added here as needed
type SplitPaneOptions struct {
	Vertical                bool
	CustomProfileProperties CustomProfileProperties
}

type session struct {
	c  *client.Client
	id string
}

func (s *session) SendText(t string) error {
	resp, err := s.c.Call(&api.ClientOriginatedMessage{
		Submessage: &api.ClientOriginatedMessage_SendTextRequest{
			SendTextRequest: &api.SendTextRequest{
				Session: &s.id,
				Text:    &t,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("error sending text to session %q: %w", s.id, err)
	}
	if status := resp.GetSendTextResponse().GetStatus(); status != api.SendTextResponse_OK {
		return fmt.Errorf("unexpected status for session %q: %s", s.id, status)
	}
	return nil
}

func (s *session) Activate(selectTab, orderWindowFront bool) error {
	resp, err := s.c.Call(&api.ClientOriginatedMessage{
		Submessage: &api.ClientOriginatedMessage_ActivateRequest{
			ActivateRequest: &api.ActivateRequest{
				Identifier: &api.ActivateRequest_SessionId{
					SessionId: s.id,
				},
				SelectTab:        &selectTab,
				OrderWindowFront: &orderWindowFront,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("error activating session %q: %w", s.id, err)
	}
	if status := resp.GetActivateResponse().GetStatus(); status != api.ActivateResponse_OK {
		return fmt.Errorf("unexpected status for activate request: %s", status)
	}
	return nil
}

func (s *session) SplitPane(opts SplitPaneOptions) (Session, error) {
	direction := api.SplitPaneRequest_HORIZONTAL.Enum()
	if opts.Vertical {
		direction = api.SplitPaneRequest_VERTICAL.Enum()
	}

	customProps, err := opts.CustomProfileProperties.toProperties()
	if err != nil {
		return nil, err
	}

	resp, err := s.c.Call(&api.ClientOriginatedMessage{
		Submessage: &api.ClientOriginatedMessage_SplitPaneRequest{
			SplitPaneRequest: &api.SplitPaneRequest{
				Session:                 &s.id,
				SplitDirection:          direction,
				CustomProfileProperties: customProps,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error splitting pane: %w", err)
	}
	spResp := resp.GetSplitPaneResponse()
	if len(spResp.GetSessionId()) < 1 {
		return nil, fmt.Errorf("expected at least one new session in split pane")
	}
	return &session{
		c:  s.c,
		id: spResp.GetSessionId()[0],
	}, nil
}

func (s *session) GetSessionID() string {
	return s.id
}

func (s *session) SetName(name string) error {
	_, err := s.c.Call(&api.ClientOriginatedMessage{
		Submessage: &api.ClientOriginatedMessage_InvokeFunctionRequest{
			InvokeFunctionRequest: &api.InvokeFunctionRequest{
				Invocation: str(fmt.Sprintf(`iterm2.set_name(name: "%s")`, name)),
				Context: &api.InvokeFunctionRequest_Method_{
					Method: &api.InvokeFunctionRequest_Method{
						Receiver: &s.id,
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("could not call set_name: %w", err)
	}
	return nil
}

func (s *session) GetVariable(name string) (string, error) {
	resp, err := s.c.Call(&api.ClientOriginatedMessage{
		Submessage: &api.ClientOriginatedMessage_VariableRequest{
			VariableRequest: &api.VariableRequest{
				Scope: &api.VariableRequest_SessionId{
					SessionId: s.id,
				},
				Get: []string{name},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to get variable %q: %w", name, err)
	}
	if resp == nil || resp.GetVariableResponse() == nil {
		return "", fmt.Errorf("failed to get variable %q: resp is nil", name)
	}
	varResp := resp.GetVariableResponse()
	if varResp.Status != nil && *varResp.Status != api.VariableResponse_OK {
		return "", fmt.Errorf("failed to get variable %q: resp status not ok (%v)", name, varResp.Status)
	}
	values := varResp.GetValues()
	if len(values) == 0 {
		return "", fmt.Errorf("failed to get variable %q: no value returned", name)
	}

	return values[0], nil
}
