package workloadattest

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/shirou/gopsutil/v4/process"
)

type UnixAttestation struct {
	Attested bool
	PID      int
	UID      int
	GID      int
}

type UnixAttestor struct {
}

func (a *UnixAttestor) Attest(ctx context.Context, pid int) (UnixAttestation, error) {
	p, err := process.NewProcessWithContext(ctx, int32(pid))
	if err != nil {
		return UnixAttestation{}, trace.Wrap(err, "getting process")
	}

	gids, err := p.Gids()
	if err != nil {
		return UnixAttestation{}, trace.Wrap(err, "getting gids")
	}

	uids, err := p.Uids()
	if err != nil {
		return UnixAttestation{}, trace.Wrap(err, "getting uids")
	}

	// TODO: Check lengths ???

	// This attestor is a little special in that we pull
	return UnixAttestation{
		Attested: true,
		PID:      pid,
		// TODO
		UID: int(uids[0]),
		// TODO
		GID: int(gids[0]),
	}, nil
}
