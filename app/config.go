package main

import (
	"gorm.io/gorm"
)

type Config struct {
	ID           uint   `gorm:"primarykey" json:"id"`
	Email        string `json:"email"`
	Domains      string `json:"domains"`
	DNSProvider  string `json:"dns_provider"`
	EnvVars      string `json:"env_vars"`
	CAServer     string `json:"ca_server"`
	KeyType      string `json:"key_type"`
	DNSResolvers string `json:"dns_resolvers"`
	EABEnabled   bool   `json:"eab_enabled"`
	EABKID       string `json:"eab_kid"`
	EABHMAC      string `json:"eab_hmac"`
}

func GetConfig(db *gorm.DB) (*Config, error) {
	var config Config
	result := db.First(&config)
	if result.Error != nil {
		return nil, result.Error
	}
	if config.DNSResolvers == "" {
		config.DNSResolvers = "8.8.8.8:53"
	}
	if config.CAServer == "" {
		config.CAServer = "https://acme-v02.api.letsencrypt.org/directory"
	}
	if config.KeyType == "" {
		config.KeyType = "ec256"
	}
	return &config, nil
}

func SaveConfig(db *gorm.DB, config *Config) error {
	if config.ID == 0 {
		return db.Create(config).Error
	}
	return db.Save(config).Error
}

func SeedDefaultConfig(db *gorm.DB) error {
	var count int64
	db.Model(&Config{}).Count(&count)
	if count > 0 {
		return nil
	}
	return db.Create(&Config{
		Email:        "",
		Domains:      "",
		EnvVars:      "{}",
		CAServer:     "https://acme-v02.api.letsencrypt.org/directory",
		KeyType:      "ec256",
		DNSResolvers: "8.8.8.8:53",
	}).Error
}
