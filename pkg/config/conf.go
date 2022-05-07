package config

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

const (
	configFileName = "config.yaml"
	dirMode        = 0700
	fileMode       = 0600
)

// Config represents app config object.
type Config struct {
	Value string    `yaml:"val"`
	Date  time.Time `yaml:"date"`
	Bool  bool      `yaml:"bool"`
	Int   int       `yaml:"int"`
}

func getDefaultConfig() *Config {
	return &Config{
		Value: "default",
		Date:  time.Now(),
		Bool:  true,
		Int:   1,
	}
}

func Save(dirPath string, c *Config) error {
	if dirPath == "" {
		return errors.New("config directory required")
	}
	if c == nil {
		return errors.New("config required")
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return errors.Wrap(err, "failed to marshal config")
	}
	path := filepath.Join(dirPath, configFileName)
	if err := os.WriteFile(path, b, fileMode); err != nil {
		return errors.Wrapf(err, "failed to write config file: %s", configFileName)
	}
	return nil
}

// ReadOrCreate reads app config from directory or creates a new one.
func ReadOrCreate(dirPath string) (*Config, error) {
	if dirPath == "" {
		return nil, errors.New("config directory required")
	}

	if _, err := os.Stat(dirPath); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(dirPath, dirMode)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create dir: %s", dirPath)
		}
	}

	path := filepath.Join(dirPath, configFileName)

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := Save(dirPath, getDefaultConfig()); err != nil {
			return nil, errors.Wrap(err, "failed to create default config")
		}
	}

	j, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening config file: %s", path)
	}
	defer j.Close()

	b, err := io.ReadAll(j)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading config file %v", j)
	}

	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, errors.Wrapf(err, "error unmarshalling config file %v", j)
	}
	return &c, nil
}

// GetOrCreateHomeDir returns the home directory for the current user.
// The create flag is set to true if the directory was created.
func GetOrCreateHomeDir(name string) (path string, created bool, err error) {
	if name == "" {
		return "", false, errors.New("name cannot be empty")
	}

	if !strings.HasPrefix(name, ".") {
		name = "." + name
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", false, errors.Wrap(err, "failed to get user home dir")
	}
	log.Debug().Msgf("home dir: %s", home)

	dir := filepath.Join(home, name)
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		log.Debug().Msgf("creating dir: %s", dir)
		err := os.Mkdir(dir, dirMode)
		if err != nil {
			return "", false, errors.Wrapf(err, "failed to create dir: %s", dir)
		}
		created = true
	}
	return dir, created, nil
}
