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

package internal

// UnversionedTeleport is used to read all versions of teleport.yaml, including
// versions that may now be unsupported.
type UnversionedTeleport struct {
	Teleport UnversionedConfig `yaml:"teleport"`
}

// UnversionedConfig is used to read unversioned configuration from teleport and tbot.
type UnversionedConfig struct {
	AuthServers []string `yaml:"auth_servers"`
	AuthServer  string   `yaml:"auth_server"`
	ProxyServer string   `yaml:"proxy_server"`
	DataDir     string   `yaml:"data_dir"`
}
