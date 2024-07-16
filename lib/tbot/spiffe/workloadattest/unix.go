package workloadattest

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/shirou/gopsutil/v4/process"
)

type UnixAttestation struct {
	// Attested is true if the PID was successfully attested to a Unix
	// process. This indicates the validity of the rest of the fields.
	Attested bool
	// PID is the process ID of the attested process.
	PID int
	// UID is the primary user ID of the attested process.
	UID int
	// GID is the primary group ID of the attested process.
	GID int
}

// LogValue implements slog.LogValue to provide a nicely formatted set of
// log keys for a given attestation.
func (a UnixAttestation) LogValue() slog.Value {
	values := []slog.Attr{
		slog.Bool("attested", a.Attested),
	}
	if a.Attested {
		values = append(values,
			slog.Int("uid", a.UID),
			slog.Int("pid", a.UID),
			slog.Int("gid", a.GID),
		)
	}
	return slog.GroupValue(values...)
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
