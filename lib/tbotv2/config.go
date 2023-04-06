package tbotv2

import (
	"bytes"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
	"os"
	"time"
)

type Config struct {
	AuthServer string `yaml:"auth_server"`
	Oneshot    bool   `yaml:"oneshot"`

	// Store requires polymorphic marshalling/unmarshalling
	Store Store `yaml:"-"`

	// For the bots own identity rather than produced certs
	TTL   time.Duration `yaml:"certificate_ttl"`
	Renew time.Duration `yaml:"renewal_interval"`

	// Destinations requires polymorphic marshalling/unmarshalling
	Destinations []Destination `yaml:"-"`
}

func (c *Config) UnmarshalYAML(node *yaml.Node) error {
	// Alias the type to get rid of the UnmarshalYAML :)
	type rawConf Config
	if err := node.Decode((*rawConf)(c)); err != nil {
		return trace.Wrap(err)
	}
	// We now have set up all the fields except those with special handling

	if err := c.unmarshalDestinations(node); err != nil {
		return trace.Wrap(err)
	}
	rawStore := struct {
		Store yaml.Node `yaml:"store"`
	}{}
	if err := node.Decode(&rawStore); err != nil {
		return trace.Wrap(err)
	}
	store, err := unmarshalStore(&rawStore.Store)
	if err != nil {
		return err
	}
	c.Store = store

	return nil
}

func (c *Config) unmarshalDestinations(node *yaml.Node) error {
	// Special handling for polymorphic unmarshalling of destinations
	rawDests := struct {
		Destinations []yaml.Node `yaml:"destinations"`
	}{}
	if err := node.Decode(&rawDests); err != nil {
		return trace.Wrap(err)
	}

	for _, rawDest := range rawDests.Destinations {
		header := struct {
			Type string `yaml:"type"`
		}{}
		if err := rawDest.Decode(&header); err != nil {
			return trace.Wrap(err)
		}

		switch header.Type {
		case "application":
			v := &ApplicationDestination{}
			if err := rawDest.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			c.Destinations = append(c.Destinations, v)
		case "identity":
			v := &IdentityDestination{}
			if err := rawDest.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			c.Destinations = append(c.Destinations, v)
		default:
			return trace.BadParameter("unrecognised destination type (%s)", header.Type)
		}
	}

	return nil
}

func unmarshalStore(raw *yaml.Node) (Store, error) {
	header := struct {
		Type string `yaml:"type"`
	}{}
	if err := raw.Decode(&header); err != nil {
		return nil, trace.Wrap(err)
	}

	switch header.Type {
	case MemoryStoreType:
		v := &MemoryStore{}
		if err := raw.Decode(v); err != nil {
			return nil, trace.Wrap(err)
		}
		return v, nil
	case DirectoryStoreType:
		v := &DirectoryStore{}
		if err := raw.Decode(v); err != nil {
			return nil, trace.Wrap(err)
		}
		return v, nil
	default:
		return nil, trace.BadParameter("unrecognised store type (%s)", header.Type)
	}
}

func (c *Config) CheckAndSetDefaults() error {
	for _, dest := range c.Destinations {
		if err := dest.CheckAndSetDefaults(); err != nil {
			return err
		}
	}
	return nil
}

func LoadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(b))
	decoder.KnownFields(true)

	var conf Config
	if err := decoder.Decode(&conf); err != nil {
		return nil, trace.Wrap(err)
	}
	return &conf, nil
}
