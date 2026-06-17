package discovery

import (
	"context"
	"io"

	"github.com/alecthomas/kingpin/v2"
)

type summaryArgs struct {
	cmd *kingpin.CmdClause
}

func (s *summaryArgs) run(ctx context.Context, clt discoveryClient, w io.Writer) error {
	// implementation later
	return nil
}
