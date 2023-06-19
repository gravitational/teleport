/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package identity

// ArtifactKind is a type of identity artifact that can be stored and loaded.
type ArtifactKind string

const (
	// KindAlways identifies identity resources that should always be
	// generated.
	KindAlways ArtifactKind = "always"

	// KindBotInternal identifies resources that should only be stored in the
	// bot's internal data directory.
	KindBotInternal ArtifactKind = "bot-internal"
)

// BotKinds returns a list of all artifact kinds used internally by the bot.
// End-user destinations may contain a different set of artifacts.
func BotKinds() []ArtifactKind {
	return []ArtifactKind{KindAlways, KindBotInternal}
}

// DestinationKinds returns a list of all artifact kinds that should be written
// to end-user destinations.
func DestinationKinds() []ArtifactKind {
	return []ArtifactKind{KindAlways}
}
