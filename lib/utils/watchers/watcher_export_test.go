// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package watchers

// WaitUntilHealthy waits until the next healthy state is communicated.
//
// The Watcher is considered healthy after receiving the init event.
//
// The healthy state is not sticky - once received the channel will be emptied,
// and will only be filled again if the underlying watcher connection closes and
// is reestablished.
//
// Useful to avoid an initial race between test and Watcher, or for testing
// reconnection scenarios, but should not be relied on to test the ongoing
// health of the watcher.
func (w *Watcher) WaitUntilHealthy() <-chan struct{} {
	return w.healthyC
}
