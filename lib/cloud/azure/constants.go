/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package azure

const (
	// MySQLPort is the Azure managed MySQL server port
	// https://docs.microsoft.com/en-us/azure/mysql/single-server/concepts-connectivity-architecture
	MySQLPort = "3306"
	// PostgresPort is the Azure managed PostgreSQL server port
	// https://docs.microsoft.com/en-us/azure/postgresql/single-server/concepts-connectivity-architecture
	PostgresPort = "5432"
	// resourceOwner is used to identify who owns the ClusterRole and ClusterRoleBinding.
	resourceOwner = "Teleport"
)
