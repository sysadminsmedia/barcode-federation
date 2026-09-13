package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Node       NodeConfig       `toml:"node"`
	Database   DatabaseConfig   `toml:"database"`
	Server     ServerConfig     `toml:"server"`
	Federation FederationConfig `toml:"federation"`
}

type NodeConfig struct {
	Host        string   `toml:"host"`
	DisplayName string   `toml:"display_name"`
	OperatedBy  string   `toml:"operated_by"`
	Contact     string   `toml:"contact"`
	KeyFile     string   `toml:"key_file"`
	TrustAnchors []string `toml:"trust_anchors"`
	SchemaRegistries []string `toml:"schema_registries"`
}

type DatabaseConfig struct {
	Path string `toml:"path"`
}

type ServerConfig struct {
	Addr string `toml:"addr"`
}

type FederationConfig struct {
	Policy    string   `toml:"policy"` // open, allowlist, blocklist
	Allowlist []string `toml:"allowlist"`
	Blocklist []string `toml:"blocklist"`
	Peers     []string `toml:"peers"` // initial peer URLs
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.setDefaults()
	return &cfg, nil
}

func (c *Config) setDefaults() {
	if c.Database.Path == "" {
		c.Database.Path = "fbs-node.db"
	}
	if c.Server.Addr == "" {
		c.Server.Addr = ":8080"
	}
	if c.Federation.Policy == "" {
		c.Federation.Policy = "open"
	}
	if c.Node.KeyFile == "" {
		c.Node.KeyFile = "node-key.pem"
	}
}

func Default() *Config {
	cfg := &Config{
		Node: NodeConfig{
			Host:        "localhost:8080",
			DisplayName: "FBS Example Node",
			OperatedBy:  "Example Operator",
		},
	}
	cfg.setDefaults()
	return cfg
}
