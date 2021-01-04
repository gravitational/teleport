package proto

import services "github.com/gravitational/teleport/lib/services"

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
