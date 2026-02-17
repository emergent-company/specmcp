package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Config holds all configuration for the SpecMCP server.
// Precedence: environment variables > config file > defaults.
type Config struct {
	Emergent  EmergentConfig  `toml:"emergent"`
	Server    ServerConfig    `toml:"server"`
	Transport TransportConfig `toml:"transport"`
	Log       LogConfig       `toml:"log"`
	Janitor   JanitorConfig   `toml:"janitor"`
}

// EmergentConfig holds Emergent connection details.
type EmergentConfig struct {
	URL        string `toml:"url"`
	Token      string `toml:"token"`       // Project-scoped token (emt_*) or standalone API key.
	AdminToken string `toml:"admin_token"` // Admin token for server-side operations (janitor, health checks) in HTTP mode.
	ProjectID  string `toml:"project_id"`  // Optional: explicit project ID (X-Project-ID header).
}

// ServerConfig holds MCP server metadata.
type ServerConfig struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`
}

// TransportConfig holds transport-related settings.
type TransportConfig struct {
	// Mode selects the transport: "stdio" (default) or "http".
	Mode string `toml:"mode"`
	// Port is the HTTP listen port (default: 21452). Only used when Mode is "http".
	Port string `toml:"port"`
	// Host is the HTTP listen address (default: "0.0.0.0"). Only used when Mode is "http".
	Host string `toml:"host"`
	// CORSOrigins is a comma-separated list of allowed CORS origins (default: "*").
	CORSOrigins string `toml:"cors_origins"`
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level string `toml:"level"` // debug, info, warn, error
}

// JanitorConfig holds janitor scheduling configuration.
type JanitorConfig struct {
	Enabled        bool `toml:"enabled"`         // Enable scheduled janitor runs
	IntervalHours  int  `toml:"interval_hours"`  // How often to run (in hours)
	CreateProposal bool `toml:"create_proposal"` // Auto-create proposals for critical issues
}

// Load creates a Config by reading from a TOML config file and environment
// variables. Precedence: environment variables > config file > defaults.
//
// Config file search order (first found wins):
//  1. Path passed via configPath parameter (from --config flag)
//  2. SPECMCP_CONFIG environment variable
//  3. ./specmcp.toml (current directory)
//  4. ~/.config/specmcp/specmcp.toml (XDG-style)
//
// All fields are optional in the config file. Environment variables always
// override file values.
func Load(configPath string) (*Config, error) {
	// Start with defaults
	cfg := &Config{
		Emergent: EmergentConfig{
			URL: "http://localhost:3002",
		},
		Server: ServerConfig{
			Name:    "specmcp",
			Version: "0.1.0",
		},
		Transport: TransportConfig{
			Mode:        "stdio",
			Port:        "21452",
			Host:        "0.0.0.0",
			CORSOrigins: "*",
		},
		Log: LogConfig{
			Level: "info",
		},
		Janitor: JanitorConfig{
			Enabled:        false, // Disabled by default
			IntervalHours:  1,     // Run every hour when enabled
			CreateProposal: false, // Don't auto-create proposals by default
		},
	}

	// Layer config file values on top of defaults
	if err := cfg.loadFile(configPath); err != nil {
		return nil, err
	}

	// Layer environment variables on top (always win)
	cfg.applyEnv()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// loadFile finds and parses the TOML config file. If no file is found,
// this is a no-op (config file is optional).
func (c *Config) loadFile(configPath string) error {
	path := resolveConfigPath(configPath)
	if path == "" {
		return nil // no config file found; rely on defaults + env
	}

	if _, err := toml.DecodeFile(path, c); err != nil {
		return fmt.Errorf("reading config file %s: %w", path, err)
	}

	return nil
}

// resolveConfigPath determines which config file to use. Returns empty string
// if no config file is found (config file is optional).
func resolveConfigPath(explicit string) string {
	// 1. Explicit path from --config flag
	if explicit != "" {
		return explicit // caller wants this file; let DecodeFile report if missing
	}

	// 2. SPECMCP_CONFIG env var
	if p := os.Getenv("SPECMCP_CONFIG"); p != "" {
		return p
	}

	// 3. ./specmcp.toml in current directory
	if _, err := os.Stat("specmcp.toml"); err == nil {
		return "specmcp.toml"
	}

	// 4. ~/.config/specmcp/specmcp.toml
	if home, err := os.UserHomeDir(); err == nil {
		p := home + "/.config/specmcp/specmcp.toml"
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}

// applyEnv overlays environment variables on top of existing config values.
// An env var only takes effect if it is non-empty.
func (c *Config) applyEnv() {
	// Emergent
	envOverride("EMERGENT_URL", &c.Emergent.URL)
	envOverride("EMERGENT_TOKEN", &c.Emergent.Token)
	envOverride("EMERGENT_API_KEY", &c.Emergent.Token) // legacy alias
	envOverride("EMERGENT_ADMIN_TOKEN", &c.Emergent.AdminToken)
	envOverride("EMERGENT_PROJECT_ID", &c.Emergent.ProjectID)

	// Transport
	envOverride("SPECMCP_TRANSPORT", &c.Transport.Mode)
	envOverride("SPECMCP_PORT", &c.Transport.Port)
	envOverride("SPECMCP_HOST", &c.Transport.Host)
	envOverride("SPECMCP_CORS_ORIGINS", &c.Transport.CORSOrigins)

	// Logging
	envOverride("SPECMCP_LOG_LEVEL", &c.Log.Level)

	// Janitor
	if v := os.Getenv("SPECMCP_JANITOR_ENABLED"); v != "" {
		c.Janitor.Enabled = (v == "true" || v == "1")
	}
	if v := os.Getenv("SPECMCP_JANITOR_INTERVAL_HOURS"); v != "" {
		var hours int
		if _, err := fmt.Sscanf(v, "%d", &hours); err == nil && hours > 0 {
			c.Janitor.IntervalHours = hours
		}
	}
	if v := os.Getenv("SPECMCP_JANITOR_CREATE_PROPOSAL"); v != "" {
		c.Janitor.CreateProposal = (v == "true" || v == "1")
	}
}

// Validate checks that required fields are present.
func (c *Config) Validate() error {
	switch c.Transport.Mode {
	case "stdio":
		// Stdio mode requires a token because there's no HTTP auth layer.
		if c.Emergent.Token == "" {
			return fmt.Errorf("emergent token is required for stdio mode: set emergent.token in config file, or EMERGENT_TOKEN env var")
		}
	case "http":
		// HTTP mode gets the token from each request's Authorization header.
		// AdminToken is optional but required for server-side operations like janitor.
		if c.Emergent.AdminToken == "" && c.Janitor.Enabled {
			return fmt.Errorf("emergent admin_token is required when janitor is enabled in HTTP mode: set emergent.admin_token in config file, or EMERGENT_ADMIN_TOKEN env var")
		}
	default:
		return fmt.Errorf("invalid transport mode: %q (must be \"stdio\" or \"http\")", c.Transport.Mode)
	}

	return nil
}

// envOverride sets *dst to the value of the named env var, if it is non-empty.
func envOverride(key string, dst *string) {
	if v := os.Getenv(key); v != "" {
		*dst = v
	}
}
