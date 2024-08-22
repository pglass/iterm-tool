package config

import (
	"embed"
	_ "embed"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed data/*.toml
var dataFS embed.FS

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name      string
		expOutput *Config
		expError  string
	}{
		{
			name: "success",
			expOutput: &Config{
				Sessions: map[string]*Session{
					"setup": {
						Name:   "setup",
						Script: "echo 'Setup is done'\n",
					},
					"server": {
						Name:      "server",
						DependsOn: []string{"sessions.setup"},
						Inject:    "echo 'This is where the server would start'\n",
					},
					"nested": {
						Name:   "nested",
						Inject: "echo 'This is the nested parent'\n",
					},
					"nested.1": {
						Name:      "nested.1",
						DependsOn: []string{"sessions.server"},
						Inject:    "echo 'This is nested 1'\n",
					},
					"nested.2": {
						Name:      "nested.2",
						DependsOn: []string{"sessions.setup"},
						Script:    "echo 'This is nested 2'\n",
					},
				},
			},
		},
		{
			name:     "unknown-field",
			expError: `unexpected field "wumbo" in sessions.setup`,
		},
		{
			name:     "empty-session",
			expError: `empty session config section sessions.empty`,
		},
		{
			name:     "unknown-field-nested",
			expError: `unexpected field "wumbo" in sessions.nested.1`,
		},
		{
			name:     "empty-session-nested",
			expError: `empty session config section sessions.nested.1`,
		},
		{
			name:     "unknown-field-nested-parent",
			expError: `unexpected field "wumbo" in sessions.nested`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fname := fmt.Sprintf("data/TestLoadConfig_%s.toml", tt.name)
			f, err := dataFS.Open(fname)
			require.NoError(t, err, fmt.Sprintf("missing test data file: config/%s (%s)", fname, err))

			cfg, err := loadConfigFile(f)

			if len(tt.expError) != 0 {
				require.ErrorContains(t, err, tt.expError)
				require.Nil(t, cfg)
			} else {
				tt.expOutput.ID = "test-load-" + tt.name
				tt.expOutput.Directory = "~/code/test-load-" + tt.name

				require.NoError(t, err)
				require.Equal(t, tt.expOutput, cfg)
			}

		})
	}

}
