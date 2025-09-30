package main

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/pelletier/go-toml/v2"
	"log"
	"os"
	"path"
	"workflowd/util"
)

type ServerConfig struct {
	Port      string `toml:"port"`
	ClusterId string `toml:"cluster_id"`
}

type Config struct {
	Server ServerConfig `toml:"server"`
}

func (c *Config) Read(fileName string) error {
	config, err := GetConfig(fileName)
	if err != nil {
		log.Fatalf("Error: %v\n", err)
		return err
	}
	*c = config
	return nil
}

// GetConfig Read a config from file
func GetConfig(fileName string) (Config, error) {
	if fileName == "" {
		currDir, err := util.GetCurrentDirectory()
		if err != nil {
			log.Fatalf("could not determine current directory: %v", err)
		}
		currenDirConfig := path.Join(currDir, "config.toml")
		config, err := readConfig(currenDirConfig)
		if err != nil {
			// 当前目录中不存在 config.toml
			homeDir, err := os.UserHomeDir()
			if err != nil {
				log.Fatalf("could not determine home directory: %v", err)
			}
			fileName = path.Join(homeDir, ".workflowd", "config.toml")
			homeDirConfig, err := readConfig(fileName)
			if err != nil {
				log.Printf("home directory not found in config file: %v", err)
				// create one
				newConfig := createDefaultConfig()
				data, err3 := toml.Marshal(newConfig)
				if err3 != nil {
					log.Printf("Error marshalling new config: %v", err3)
				} else {
					log.Printf("create a new config file : %s", currenDirConfig)
					_ = os.WriteFile(currenDirConfig, data, 0644)
				}
				return newConfig, nil
			}
			return homeDirConfig, nil
		}
		return config, err
	} else {
		return readConfig(fileName)
	}

}

// read config
func readConfig(fileName string) (Config, error) {
	// read from file
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return Config{}, err
	}
	data, err := os.ReadFile(fileName)
	if err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		return Config{}, err
	}

	var cfg Config
	err = toml.Unmarshal(data, &cfg)
	if err != nil {
		fmt.Printf("Error unmarshaling TOML: %v\n", err)
		return Config{}, err
	}
	return cfg, nil
}

func createDefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Port:      "0.0.0.0:50051",
			ClusterId: uuid.New().String(),
		},
	}
}
