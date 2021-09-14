#!/bin/sh

ufw limit ssh

# HTTPS port clients connect to. Used to authenticate tsh users and web users into the cluster.
ufw allow 443

# SSH port to the Node Service. This is Teleport's equivalent of port 22 for SSH.
ufw allow 3022

# SSH port clients connect to after authentication. A proxy will forward this connection to port 3022 on the destination node.
ufw allow 3023

# SSH port used to create "reverse SSH tunnels" from behind-firewall environments into a trusted proxy server.
ufw allow 3024

# SSH port used by the Auth Service to serve its Auth API to other nodes in a cluster.
ufw allow 3025

ufw --force enable
