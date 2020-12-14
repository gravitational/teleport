package proto

import (
	"time"

	"github.com/gravitational/teleport/lib/services"
)

// Duration is a wrapper around duration to set up custom marshal/unmarshal
type Duration time.Duration

// Get returns time.Duration value
func (d Duration) Get() time.Duration {
	return time.Duration(d)
}

// Set sets time.Duration value
func (d *Duration) Set(value time.Duration) {
	*d = Duration(value)
}

// WatchKindToProto converts the watch kind value between internal
// and the protobuf format
func WatchKindToProto(wk services.WatchKind) WatchKind {
	return WatchKind{
		Name:        wk.Name,
		Kind:        wk.Kind,
		SubKind:     wk.SubKind,
		LoadSecrets: wk.LoadSecrets,
		Filter:      wk.Filter,
	}
}

// ProtoToWatchKind converts the watch kind value between the protobuf
// and the internal format
func ProtoToWatchKind(wk WatchKind) services.WatchKind {
	return services.WatchKind{
		Name:        wk.Name,
		Kind:        wk.Kind,
		SubKind:     wk.SubKind,
		LoadSecrets: wk.LoadSecrets,
		Filter:      wk.Filter,
	}
}
