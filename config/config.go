package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/hashicorp/go-multierror"
)

type Config struct {
	ID        string `validate:"required"`
	Directory string
	Sessions  map[string]*Session `validate:"gte=1"`
}

func (c Config) Validate() error {
	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(c); err != nil {
		return err
	}

	var errs error
	for _, s := range c.Sessions {
		if err := s.Validate(); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

func (c Config) SessionsByGroup() map[string][]*Session {
	result := map[string][]*Session{}
	for _, sess := range c.Sessions {
		list := result[sess.Group()]
		list = append(list, sess)
		result[sess.Group()] = list
	}

	for _, list := range result {
		sort.Slice(list, func(i, j int) bool {
			return list[i].Name < list[j].Name
		})

	}
	return result
}

type Session struct {
	Name      string
	DependsOn []string `mapstructure:"depends_on"`
	Script    string
	Inject    string
}

func (s Session) Validate() error {
	if s.Script == "" && s.Inject == "" {
		return fmt.Errorf("in session %q: one of script or inject is required", s.Name)
	}
	return nil
}

// Group is a session grouping.
// We infer the group from the name and use it to control layout (vsplit vs hsplit).
//
// We use ':' to delimit group name:
// - `session.<name>` (no group)
// - `session.<group>:<name>` (yes group)
//
// To simplify some logic, default the group name to the session name.
func (s Session) Group() string {
	parts := strings.SplitN(s.Name, ".", 2)
	if len(parts) < 2 {
		return s.Name
	}
	parts = strings.SplitN(s.Name, ".", 2)
	if len(parts) < 2 {
		return s.Name
	}
	return parts[0]
}
