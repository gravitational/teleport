TELEBOX=teleport:latest
HOMEDIR=$(abspath ..)
CONTAINERHOME=/root/go/src/github.com/gravitational/teleport 
THISDIR=`pwd`
NETNAME=telenet
DOCKEROPS=--net $(NETNAME) -w $(CONTAINERHOME) -v $(HOMEDIR):$(CONTAINERHOME)

#
# Default target starts two Teleport clusters
#
.PHONY:run
run: 
	# create a docker Teleport image and a network
	docker build -t $(TELEBOX) .
	docker network create --subnet=172.10.0.0/16 $(NETNAME)
	mkdir -p data/one data/two/proxy data/two/node data/two/auth
	# start the single-node cluster named "one"
	docker run --name=one --detach=true \
		--hostname one \
		--ip 172.10.1.1 \
		--publish 3080:3080 -p 3023:3023 \
		--volume $(THISDIR)/data/one:/var/lib/teleport \
		$(DOCKEROPS) $(TELEBOX) build/teleport start -c /teleport/docker/one.yaml
	# start three-node cluster named "two"
	docker run --name=two-auth --detach=true \
		--hostname two-auth \
		--ip 172.10.1.2 \
		--volume $(THISDIR)/data/two/auth:/var/lib/teleport \
		$(DOCKEROPS) $(TELEBOX) build/teleport start -c /teleport/docker/two-auth.yaml
	docker run --name=two-proxy --detach=true \
		--hostname two-proxy \
		--ip 172.10.1.3 \
		--publish 5080:5080 -p 5023:5023 \
		--volume $(THISDIR)/data/two/proxy:/var/lib/teleport \
		$(DOCKEROPS) $(TELEBOX) build/teleport start -c /teleport/docker/two-proxy.yaml
	docker run --name=two-node --detach=true \
		--hostname two-node \
		--ip 172.10.1.4 \
		--volume $(THISDIR)/data/two/node:/var/lib/teleport \
		$(DOCKEROPS) $(TELEBOX) build/teleport start -c /teleport/docker/two-node.yaml

# 'make stop' stops all Teleport containers, deletes them 
# and their network
#
.PHONY:stop
stop:
	-@docker rm -f one two-auth two-proxy two-node
	-@docker network rm $(NETNAME)

# `make enter-one` gives you shell inside auth server 
# of cluster "one"
#
.PHONY:enter-one
enter-one:
	docker exec -ti one /bin/bash

# `make enter-two` gives you shell inside auth server 
# of cluster "two"
#
.PHONY:enter-two
enter-two:
	docker exec -ti two-auth /bin/bash


# `make shell` drops you into a bash shell inside an empty container, 
# without Teleport running. Useful if you want to start it manually 
# from the inside
.PHONY:shell
shell: 
	docker build -t $(TELEBOX) .
	-docker network create --subnet=172.10.0.0/16 $(NETNAME)
	-docker run --name=one --rm=true -ti \
		--hostname one \
		--ip 172.10.1.1 \
		--volume $(THISDIR)/data/one:/var/lib/teleport \
		$(DOCKEROPS) $(TELEBOX) /bin/bash
	-docker network rm $(NETNAME)
