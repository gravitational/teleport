/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package cli

type KubeCredentialsCommand struct {
	*genericExecutorHandler[KubeCredentialsCommand]

	DestinationDir string
}

// NewKubeCredentialsCommand initializes a kubernetes `credentials` command
// and returns a struct that will contain the parse result.
func NewKubeCredentialsCommand(parentCmd KingpinClause, action func(*KubeCredentialsCommand) error) *KubeCredentialsCommand {
	cmd := parentCmd.Command("credentials", "Get credentials for kubectl access").Hidden()

	c := &KubeCredentialsCommand{}
	c.genericExecutorHandler = newGenericExecutorHandler(cmd, c, action)

	cmd.Flag("destination-dir", "The destination directory with which to generate Kubernetes credentials").Required().StringVar(&c.DestinationDir)

	return c
}
