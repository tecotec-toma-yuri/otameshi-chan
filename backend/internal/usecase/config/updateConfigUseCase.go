package config

import cfg "github.com/otameshi/backend/internal/config"

func UpdateConfig(c cfg.Config) error {
	return cfg.Update(c)
}
