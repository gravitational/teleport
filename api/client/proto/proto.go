package proto

import services "github.com/gravitational/teleport/lib/services"

// FromWatchKind converts the watch kind value between internal
// and the protobuf format
func FromWatchKind(wk services.WatchKind) WatchKind {
	return WatchKind{
		Name:        wk.Name,
		Kind:        wk.Kind,
		SubKind:     wk.SubKind,
		LoadSecrets: wk.LoadSecrets,
		Filter:      wk.Filter,
	}
}

// ToWatchKind converts the watch kind value between the protobuf
// and the internal format
func ToWatchKind(wk WatchKind) services.WatchKind {
	return services.WatchKind{
		Name:        wk.Name,
		Kind:        wk.Kind,
		SubKind:     wk.SubKind,
		LoadSecrets: wk.LoadSecrets,
		Filter:      wk.Filter,
	}
}
