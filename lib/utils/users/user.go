/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package users

import "fmt"

// UsernameForRemoteCluster returns an username that is prefixed with "remote-"
// and suffixed with cluster name with the hope that it does not match a real
// local user.
func UsernameForRemoteCluster(localUsername, localClusterName string) string {
	return fmt.Sprintf("remote-%v-%v", localUsername, localClusterName)
}

func UsernameForCluster(username, localClusterName, userClusterName string) string {
	if localClusterName == userClusterName {
		return username
	}
	return UsernameForRemoteCluster(username, userClusterName)
}
