package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	URL       string `json:"url,omitempty"`
	Token     string `json:"token,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
}

type Paths struct {
	Global string
	Local  string
}

func DefaultPaths(cwd string) (Paths, error) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("find user config dir: %w", err)
	}
	if cwd == "" {
		cwd, err = os.Getwd()
		if err != nil {
			return Paths{}, fmt.Errorf("find working dir: %w", err)
		}
	}
	return Paths{
		Global: filepath.Join(userConfigDir, "ytrack", "config.json"),
		Local:  filepath.Join(cwd, ".ytrack", "config.json"),
	}, nil
}

func Load(path string) (Config, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(content))) == 0 {
		return Config{}, nil
	}

	var cfg Config
	if err := json.Unmarshal(content, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	content = append(content, '\n')

	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return os.Chmod(path, 0o600)
}

func LoadEffective(paths Paths) (Config, error) {
	global, err := Load(paths.Global)
	if err != nil {
		return Config{}, err
	}
	local, err := Load(paths.Local)
	if err != nil {
		return Config{}, err
	}
	return Merge(global, local), nil
}

func Merge(global, local Config) Config {
	merged := Config{
		URL:   global.URL,
		Token: global.Token,
	}
	if local.URL != "" {
		merged.URL = local.URL
	}
	if local.Token != "" {
		merged.Token = local.Token
	}
	merged.ProjectID = local.ProjectID
	return merged
}

func MaskToken(token string) string {
	if token == "" {
		return ""
	}

	prefix := ""
	secret := token
	if idx := strings.Index(token, ":"); idx >= 0 {
		prefix = token[:idx+1]
		secret = token[idx+1:]
	}
	if len(secret) <= 4 {
		return prefix + "xxxx"
	}
	return prefix + "xxxx..." + secret[len(secret)-4:]
}

func Require(cfg Config, fields ...string) error {
	for _, field := range fields {
		switch field {
		case "url":
			if cfg.URL == "" {
				return errors.New("missing configured url, run `ytrack set-url <url>` or `ytrack global set-url <url>`")
			}
		case "token":
			if cfg.Token == "" {
				return errors.New("missing configured token, run `ytrack set-token <token>` or `ytrack global set-token <token>`")
			}
		case "project_id":
			if cfg.ProjectID == "" {
				return errors.New("missing configured project_id, run `ytrack set-project-id <project-id>`")
			}
		default:
			return fmt.Errorf("unknown required config field %q", field)
		}
	}
	return nil
}
