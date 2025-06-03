package collections

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"io"
)

type tokenCollection struct {
	tokens []types.ProvisionToken
}

func NewTokenCollection(tokens []types.ProvisionToken) ResourceCollection {
	return &tokenCollection{tokens: tokens}
}

func (c *tokenCollection) Resources() (r []types.Resource) {
	for _, resource := range c.tokens {
		r = append(r, resource)
	}
	return r
}

func (c *tokenCollection) WriteText(w io.Writer, verbose bool) error {
	for _, token := range c.tokens {
		_, err := w.Write([]byte(token.String()))
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
