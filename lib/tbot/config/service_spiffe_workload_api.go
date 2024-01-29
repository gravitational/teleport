package config

import (
	"fmt"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
)

const SPIFFEWorkloadAPIServiceType = "spiffe-workload-api"

// SPIFFEWorkloadAPIService is the configuration for the SPIFFE Workload API
// service.
type SPIFFEWorkloadAPIService struct {
	// Listen is the address on which the SPIFFE Workload API server should
	// listen. This should either be prefixed with "unix://" or "tcp://".
	Listen string        `yaml:"listen"`
	SVIDs  []SVIDRequest `yaml:"svids"`
}

func (s *SPIFFEWorkloadAPIService) Type() string {
	return SPIFFEWorkloadAPIServiceType
}

func (s *SPIFFEWorkloadAPIService) MarshalYAML() (interface{}, error) {
	type raw SPIFFEWorkloadAPIService
	return withTypeHeader((*raw)(s), SPIFFEWorkloadAPIServiceType)
}

func (s *SPIFFEWorkloadAPIService) UnmarshalYAML(node *yaml.Node) error {
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw SPIFFEWorkloadAPIService
	if err := node.Decode((*raw)(s)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *SPIFFEWorkloadAPIService) String() string {
	return fmt.Sprintf("%s", SPIFFEWorkloadAPIServiceType)
}

func (s *SPIFFEWorkloadAPIService) CheckAndSetDefaults() error {
	if s.Listen == "" {
		return trace.BadParameter("listen: should not be empty")
	}
	if len(s.SVIDs) == 0 {
		return trace.BadParameter("svids: should not be empty")
	}
	for i, svid := range s.SVIDs {
		if err := svid.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err, "validiting svid[%d]", i)
		}
	}
	return nil
}
