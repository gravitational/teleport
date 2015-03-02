package srv

import (
	"strings"

	"github.com/gravitational/teleport/backend"
)

// resolver is an interface implementing a query resolver,
// it get's a query and resolves it into a list of address strings
type resolver interface {
	resolve(query string) ([]string, error)
}

// backend resolver is a simple implementation of the resolver
// that uses servers presence information to find the servers
type backendResolver struct {
	b backend.Backend
}

// resolve provides a simple demo resolve functionality,such as globbing, expanding to all hosts
// or accepting a list of hosts:port pairs
func (b *backendResolver) resolve(query string) ([]string, error) {
	// simply expand the query to all known hosts
	if query == "*" {
		out := []string{}
		srvs, err := b.b.GetServers()
		if err != nil {
			return nil, err
		}
		for _, s := range srvs {
			out = append(out, s.Addr)
		}
		return out, nil
	}
	// else treat it as a list of hosts
	return strings.Split(query, ","), nil
}
