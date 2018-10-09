FROM ubuntu:18.10

RUN apt-get update && apt-get install -y \
    dumb-init \
 && rm -rf /var/lib/apt/lists/*

# Bundle teleport and control binary
ADD teleport /usr/local/bin/teleport
ADD tctl /usr/local/bin/tctl

# By setting this entry point, we expose make target as command
ENTRYPOINT ["/usr/bin/dumb-init", "teleport", "start", "-c", "/etc/teleport/teleport.yaml"]
