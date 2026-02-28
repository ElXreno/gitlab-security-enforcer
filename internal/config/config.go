package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

const defaultListenAddr = ":8080"

type Config struct {
	GitLabURL   string `env:"GITLAB_URL,required"`
	GitLabToken string `env:"GITLAB_TOKEN,required"`
	HookSecret  string `env:"HOOK_SECRET,required"`
	ListenAddr  string `env:"LISTEN_ADDR" envDefault:":8080"`
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultListenAddr
	}
	return cfg, nil
}
