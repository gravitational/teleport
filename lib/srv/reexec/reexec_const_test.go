package reexec

import (
	"github.com/gravitational/teleport"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/sshutils"
)

func _() {
	const mustBeTrue = teleport.RemoteCommandSuccess == RemoteCommandSuccess
	_ = map[bool]struct{}{false: {}, mustBeTrue: {}}
}
func _() {
	const mustBeTrue = teleport.RemoteCommandFailure == RemoteCommandFailure
	_ = map[bool]struct{}{false: {}, mustBeTrue: {}}
}
func _() {
	const mustBeTrue = teleport.HomeDirNotFound == HomeDirNotFound
	_ = map[bool]struct{}{false: {}, mustBeTrue: {}}
}
func _() {
	const mustBeTrue = teleport.HomeDirNotAccessible == HomeDirNotAccessible
	_ = map[bool]struct{}{false: {}, mustBeTrue: {}}
}
func _() {
	const mustBeTrue = teleport.UnexpectedCredentials == UnexpectedCredentials
	_ = map[bool]struct{}{false: {}, mustBeTrue: {}}
}

func _() {
	const mustBeTrue = teleport.ExecSubCommand == ExecSubCommand
	_ = map[bool]struct{}{false: {}, mustBeTrue: {}}
}
func _() {
	const mustBeTrue = teleport.NetworkingSubCommand == NetworkingSubCommand
	_ = map[bool]struct{}{false: {}, mustBeTrue: {}}
}
func _() {
	const mustBeTrue = teleport.CheckHomeDirSubCommand == CheckHomeDirSubCommand
	_ = map[bool]struct{}{false: {}, mustBeTrue: {}}
}
func _() {
	const mustBeTrue = teleport.ParkSubCommand == ParkSubCommand
	_ = map[bool]struct{}{false: {}, mustBeTrue: {}}
}
func _() {
	const mustBeTrue = teleport.SFTPSubCommand == SFTPSubCommand
	_ = map[bool]struct{}{false: {}, mustBeTrue: {}}
}

func _() {
	const mustBeTrue = sshutils.ShellRequest == ShellRequest
	_ = map[bool]struct{}{false: {}, mustBeTrue: {}}
}
func _() {
	const mustBeTrue = sshutils.SubsystemRequest == SubsystemRequest
	_ = map[bool]struct{}{false: {}, mustBeTrue: {}}
}
func _() {
	const mustBeTrue = teleport.SFTPSubsystem == SFTPSubsystem
	_ = map[bool]struct{}{false: {}, mustBeTrue: {}}
}

func _() {
	const mustBeTrue = apitypes.TeleportDropGroup == TeleportDropGroup
	_ = map[bool]struct{}{false: {}, mustBeTrue: {}}
}
