FROM quay.io/gravitational/debian-grande:0.0.1

# Bundle teleport and control binary
ADD teleport /usr/local/bin/teleport
ADD tctl /usr/local/bin/tctl

# By setting this entry point, we expose make target as command
ENTRYPOINT ["/usr/bin/dumb-init", "teleport", "start", "-c", "/etc/teleport/teleport.yaml"]
