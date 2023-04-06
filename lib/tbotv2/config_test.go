package tbotv2

import (
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"strings"
	"testing"
)

func TestConfig_Unmarshal(t *testing.T) {
	data := `
auth_server: tele.example.com:443
renewal_interval: 5m
certificate_ttl: 10m
store:
  type: memory
destinations:
- type: application
  name: httpbin
  store:
    type: memory
  roles:
  - access
- type: identity
  store:
    type: directory
    path: ./identity-out
  roles:
  - editor
  - access
`
	dec := yaml.NewDecoder(strings.NewReader(data))
	c := Config{}
	err := dec.Decode(&c)
	require.NoError(t, err)
	require.Equal(t, "tele.example.com:443", c.AuthServer)
}
