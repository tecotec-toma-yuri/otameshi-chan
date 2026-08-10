package config

import cfg "github.com/otameshi/backend/internal/config"

func GetConfig() cfg.Config {
	return cfg.Get()
}
