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

// How do we fit oneshot operation with long-haul operation without
// making things ugly. Can we skip CA watching etc etc when operating as a
// oneshot ???

// How do we mix bot identities - all are reusable bar the static token
// Could we use an interface here ??

// CA rotations:
// - Prioritise bot's own identity first ? Or renew this concurrently with
//   consumer identities??

// Is a thing that maintains its own identity universally useful ?
// [bot identity] -> [consumer identity]
//                   [consumer identity]
//                   [consumer identity]
