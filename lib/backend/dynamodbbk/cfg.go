package dynamodbbk

import (
	"encoding/json"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// Config represents JSON config for dynamodb backend
type Config struct {
	Tablename string `json:"tablename"`
	Region    string `json:"region"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

// Check checks if all the parameters are valid
func (cfg *Config) Check() error {
	if len(cfg.Tablename) == 0 {
		return trace.BadParameter(`Tablename: supply the tablename where Teleport data are stored`)
	}
	return nil
}

// FromObject initialized the backend from backend-specific string
func FromObject(in interface{}) (backend.Backend, error) {
	var cfg *Config
	if err := utils.ObjectToStruct(in, &cfg); err != nil {
		return nil, trace.Wrap(err)
	}
	return New(*cfg)
}

// FromJSON returns backend initialized from JSON-encoded string
func FromJSON(paramsJSON string) (backend.Backend, error) {
	cfg := Config{}
	err := json.Unmarshal([]byte(paramsJSON), &cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return New(cfg)
}
