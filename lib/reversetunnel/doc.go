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

/*
Package reversetunnel provides interfaces for accessing remote clusters

	via reverse tunnels and directly.

Reverse Tunnels

	Proxy server                      Proxy agent
	               Reverse tunnel
	+----------+                      +---------+
	|          <----------------------+         |
	|          |                      |         |

+-----+----------+                      +---------+-----+
|                |                      |               |
|                |                      |               |
+----------------+                      +---------------+

	Proxy Cluster "A"                      Proxy Cluster "B"

Reverse tunnel is established from a cluster "B" Proxy
to the a cluster "A" proxy, and clients of the cluster "A"
can access servers of the cluster "B" via reverse tunnel connection,
even if the cluster "B" is behind the firewall.

# Multiple Proxies and Revese Tunnels

With multiple proxies behind the load balancer,
proxy agents will eventually discover and establish connections to all
proxies in cluster.

* Initially Proxy Agent connects to Proxy 1.
* Proxy 1 starts sending information about all available proxies
to the Proxy Agent . This process is called "sending discovery request".

+----------+
|          <--------+
|          |        |
+----------+        |     +-----------+             +----------+

	Proxy 1           +-------------------------------+          |
	                        |           |             |          |
	                        +-----------+             +----------+
	                         Load Balancer             Proxy Agent

+----------+
|          |
|          |
+----------+

	Proxy 2

* Agent will use the discovery request to establish new connections
and check if it has connected and "discovered" all the proxies specified

	in the discovery request.

* Assuming that load balancer uses fair load balancing algorithm,
agent will eventually discover and connect back to all the proxies.

+----------+
|          <--------+
|          |        |
+----------+        |     +-----------+             +----------+

	Proxy 1           +-------------------------------+          |
	                  |     |           |             |          |
	                  |     +-----------+             +----------+
	                  |      Load Balancer             Proxy Agent

+----------+        |
|          <--------+
|          |
+----------+

	Proxy 2
*/
package reversetunnel
