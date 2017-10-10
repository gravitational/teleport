/*
Copyright 2015 Gravitational, Inc.

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

/* package reversetunnel provides interfaces for accessing remote clusters
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

Multiple Proxies and Revese Tunnels

With multiple proxies behind the load balancer,
proxy agents will eventually discover and establish connections to all
proxies in cluster.

* Initially Proxy Agent connects to Proxy 1.
* Proxy 1 starts sending information about all available proxies
that have not received connection from the Proxy Agent yet. This
process is called "sending discovery request".


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
