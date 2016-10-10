# Local Cluster

This directory contains a sample configuration of a 3-node Teleport cluster
where all 3 components are running as 3 independent processes:

* Auth : configured with static host tokens
* Proxy: configured to join 'auth'
* Node : configured to join 'auth'

This is also useful for Teleport development: open all 3 directories in 
3 different tabs and run `./start.sh` in each.
