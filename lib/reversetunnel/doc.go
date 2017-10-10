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

/* package reversetunnel provides tools for accessing remote clusters
   via reverse tunnels and directly

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


Reverse tunnel is established from the cluster "B" Proxy
to the cluster "A" proxy, and clients of cluster "A"
can access servers of cluster "B" via reverse tunnel,
even if the cluster "B" is behind the firewall.

Multiple Proxies Design

With multiple proxies behind the load balancer,
proxy agents will eventually discover and establish connections to all
proxies in a cluster.

* Initially Proxy Agent connects to the Proxy 1.
* Proxy 1 starts sending information about all the other proxies
in the cluster and whether they proxies have received the connection
from the agent or not.


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

* Agent will use this information to establish new connections
and check if it connected and "discovered" all the proxies.
* Assuming that load balancer uses fair load balancing algorithm,
agent will eventually discover all the proxies and connect back to them all

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
