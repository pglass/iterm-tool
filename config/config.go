package config

import (
	"fmt"

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

type Session struct {
	Name      string
	DependsOn []string `toml:"depends_on"`
	Script    string
	Inject    string
}

func (s Session) Validate() error {
	if s.Script == "" && s.Inject == "" {
		return fmt.Errorf("in session %q: one of script or inject is required", s.Name)
	}
	return nil
}
