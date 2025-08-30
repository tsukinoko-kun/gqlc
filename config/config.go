package config

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

const configFileName = "gqlc."

type (
	Config struct {
		Input  Input  `yaml:"input" json:"input" toml:"input" xml:"input"`
		Output Output `yaml:"output" json:"output" toml:"output" xml:"output"`
	}

	Input struct {
		Schemas          string `yaml:"schemas" json:"schemas" toml:"schemas" xml:"schemas"`
		Operations       string `yaml:"operations" json:"operations" toml:"operations" xml:"operations"`
		WebAuthorization string `yaml:"authorization,omitempty" json:"authorization,omitempty" toml:"authorization,omitempty" xml:"authorization,omitempty"`
	}

	Output struct {
		Location string `yaml:"location" json:"location" toml:"location" xml:"location"`
		Language string `yaml:"language" json:"language" toml:"language" xml:"language"`
		Package  string `yaml:"package,omitempty" json:"package,omitempty" toml:"package,omitempty" xml:"package,omitempty"`
		Suffix   string `yaml:"suffix" json:"suffix" toml:"suffix" xml:"suffix"`
	}
)

func New() *Config {
	return &Config{
		Input: Input{
			Schemas:    "graphql/schemas",
			Operations: "graphql/operations",
		},
		Output: Output{
			Location: "graphql",
			Language: "typescript",
			Package:  "",
			Suffix:   "_gqlc",
		},
	}
}

func Load() (Config, error) {
	config, err := loadFile()
	if err != nil {
		return Config{}, err
	}
	err = config.Validate()
	return config, err
}

func (c Config) SaveAs(ext string) error {
	switch ext {
	case "yaml", "yml":
		return c.saveAsYaml(ext)
	case "toml":
		return c.saveAsToml()
	case "json":
		return c.saveAsJson()
	case "xml":
		return c.saveAsXml()
	default:
		return fmt.Errorf("unsupported extension: %s", ext)
	}
}

func loadFile() (Config, error) {
	file, ok := findFile()
	if !ok {
		return Config{}, os.ErrNotExist
	}
	defer file.Close()

	switch filepath.Ext(file.Name()) {
	case ".yaml", ".yml":
		return loadYaml(file)
	case ".toml":
		return loadToml(file)
	case ".json":
		return loadJson(file)
	case ".xml":
		return loadXml(file)
	default:
		return Config{}, os.ErrInvalid
	}
}

func findFile() (*os.File, bool) {
	if yamlFile, err := os.Open(configFileName + "yaml"); err == nil {
		return yamlFile, true
	}
	if ymlFile, err := os.Open(configFileName + "yml"); err == nil {
		return ymlFile, true
	}
	if tomlFile, err := os.Open(configFileName + "toml"); err == nil {
		return tomlFile, true
	}
	if jsonFile, err := os.Open(configFileName + "json"); err == nil {
		return jsonFile, true
	}
	if xmlFile, err := os.Open(configFileName + "xml"); err == nil {
		return xmlFile, true
	}
	return nil, false
}

func loadYaml(file *os.File) (Config, error) {
	var config Config
	err := yaml.NewDecoder(file).Decode(&config)
	return config, err
}

func loadToml(file *os.File) (Config, error) {
	var config Config
	_, err := toml.NewDecoder(file).Decode(&config)
	return config, err
}

func loadJson(file *os.File) (Config, error) {
	var config Config
	err := json.NewDecoder(file).Decode(&config)
	return config, err
}

func loadXml(file *os.File) (Config, error) {
	var config Config
	err := xml.NewDecoder(file).Decode(&config)
	return config, err
}

func (c Config) Validate() error {
	if c.Input.Schemas == "" {
		return errors.New("input.schemas is required")
	}
	if c.Input.Operations == "" {
		return errors.New("input.operations is required")
	}
	if c.Output.Location == "" {
		return errors.New("output.location is required")
	}
	if c.Output.Language == "" {
		return errors.New("output.language is required")
	}
	return nil
}

func (c Config) saveAsYaml(ext string) error {
	file, err := os.Create(configFileName + ext)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(4)
	return encoder.Encode(c)
}

func (c Config) saveAsToml() error {
	file, err := os.Create(configFileName + "toml")
	if err != nil {
		return err
	}
	defer file.Close()
	return toml.NewEncoder(file).Encode(c)
}

func (c Config) saveAsJson() error {
	file, err := os.Create(configFileName + "json")
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(c)
}

func (c Config) saveAsXml() error {
	file, err := os.Create(configFileName + "xml")
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := xml.NewEncoder(file)
	encoder.Indent("", "  ")
	return encoder.Encode(c)
}

func (o Output) FileExtension() string {
	switch strings.ToLower(o.Language) {
	case "typescript", "ts":
		return "ts"
	case "typescriptreact", "tsx":
		return "tsx"
	case "go", "golang":
		return "go"
	default:
		panic("unsupported language: " + o.Language)
	}
}
