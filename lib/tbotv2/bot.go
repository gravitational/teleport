package tbotv2

type Bot struct {
	ism *IdentityStreamManager
}

type DestinationFile struct {
	IdentityRequest
	Path string
}

// IdentityStreamManager -> IdentityStream -> Formatter -> Destination ??
// Seperation of formatter and destination ?
// Destination - file, memory(?), kubernetes ??
// Formatter - SSH Config, Identity File??
// Will too much seperation/concept of pipeline be confusing ?

// How to handle bots's own identity ? Can IdentityStream mechanism be
// re-used with a join mechanism ? Or can we write a similar, but separate slice
// of code for this.

// BotIdentityManager -> Client -> IdentityStreamManager
//  ^                           -> CAWatcher --^
//  ------------------------------<|
//
// CAWatcher requires valid client, BotIdentityManager requires knowledge
// of CA rotations. This produces a cyclic dependency. CAW must also inform
// ISM of CA rotations so all outstanding IdentityStreams can be generated.

// Do we completely remove the concept of destinations from the core and make
// these a thing assembled by the command/config parser ? How should
// destinations plug with IdentityStream() ?
