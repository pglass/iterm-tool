package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/go-viper/mapstructure/v2"
	"github.com/pelletier/go-toml/v2"
)

func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return loadConfigFile(f)
}

func loadConfigFile(f fs.File) (*Config, error) {
	var rawConfig map[string]interface{}
	if err := toml.NewDecoder(f).Decode(&rawConfig); err != nil {
		return nil, err
	}

	// This is a little silly and kind of a self-imposed problem. Maybe there's a better way.
	//
	// Basically, I want to use `session.<group>.name` syntax.
	// But, TOML interprets the '.' separator as a nested table and parses it as nested maps.
	//
	// For example, this defines a few nested maps:
	//
	// ```toml
	// [a]
	// key = "val-a"
	//
	// [a.b]
	// key = "val-b"
	//
	// [a.b.c]
	// key = "val-c"
	// ```
	//
	// This parses as nested maps:
	//
	// {"a": {
	//   "key": "val-a",
	//   "b": {
	//     "key": "val-b",
	//     "c": {
	//       "key": "val-c"}}}
	//
	// Instead, I want to parse this in a "flattened" structure without nesting
	//
	// {"a": {"key": "val-a"},
	//  "a.b": {"key": "val-b"},
	//  "a.b.c": {"key": "val-c"},
	//
	// To achieve this, I parse as a raw map[string]interface{}. I pop the "sessions"
	// field out, and then parse that with special handling.
	rawSessions, ok := rawConfig["sessions"].(map[string]interface{})
	if !ok || len(rawSessions) == 0 {
		return nil, errors.New("must have at least one session config: `[sessions.<name>]`")
	}
	delete(rawConfig, "sessions")

	// Parse the top-level fields without "sessions"
	var cfg Config
	if err := mapstructure.Decode(rawConfig, &cfg); err != nil {
		return nil, err
	}

	// Parse the sessions.
	cfg.Sessions = map[string]*Session{}
	type KeyVal struct {
		Key string
		Val interface{}
	}

	// We may encounter multiple-levels of nested tables.
	// Remaining is our "work list". When we think we have a nested session config section,
	// append to to be parsed later (doing this iteratively instead of using recursion)
	remaining := []KeyVal{}
	for name, rawSess := range rawSessions {
		remaining = append(remaining, KeyVal{
			Key: name,
			Val: rawSess,
		})
	}

	for i := 0; i < len(remaining); i++ {
		kv := remaining[i]
		var sess Session

		var metadata mapstructure.Metadata
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			Metadata: &metadata,
			Result:   &sess,
		})
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(kv.Val); err != nil {
			return nil, err
		}

		if len(metadata.Keys) > 0 {
			// A valid key was parsed.
			cfg.Sessions[kv.Key] = &sess
		}

		// We may have `sessions.nested` and `sessions.nested.1` and `sessions.nested.2`.
		// Try to parse any nested sessions.
		rawSessAsMap, ok := kv.Val.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("session config %q is not a map: val=%v", kv.Key, kv.Val)
		}
		if len(rawSessAsMap) == 0 {
			return nil, fmt.Errorf("empty session config section sessions.%s", kv.Key)
		}

		for _, unused := range metadata.Unused {
			// mapstructure can distinguish unused fields and unset fields.
			// Unset fields are fields in the result type, while unused fields are the "unexpected" fields.
			//
			// We iterate over unused fields and assume that if they are maps, then we have a nested session.
			nestSess, ok := rawSessAsMap[unused].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("unexpected field %q in sessions.%s", unused, kv.Key)
			}
			remaining = append(remaining, KeyVal{
				Key: kv.Key + "." + unused,
				Val: nestSess,
			})
		}
	}

	// Sync/Surface session name.
	for name, s := range cfg.Sessions {
		s.Name = name
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validateConfig(cfg Config) error {
	return cfg.Validate()
}
