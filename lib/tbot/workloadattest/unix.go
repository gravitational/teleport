package workloadattest

import (
	"context"
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
	// This attestor is a little special in that we pull
	return UnixAttestation{
		Attested: true,
		PID:      pid,
		// TODO
		UID: 0,
		// TODO
		GID: 0,
	}, nil
}
