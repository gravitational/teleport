TELEBOX=teleport:latest
HOMEDIR=$(abspath ..)
THISDIR=`pwd`
NETNAME=telenet
DOCKEROPS=--detach=true --net $(NETNAME) -w /teleport -v $(HOMEDIR):/teleport 

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
	docker run --name=one \
		--hostname one \
		--ip 172.10.1.1 \
		--publish 3080:3080 -p 3023:3023 \
		--volume $(THISDIR)/data/one:/var/lib/teleport \
		$(DOCKEROPS) $(TELEBOX) build/teleport start -c /teleport/docker/one.yaml
	# start three-node cluster named "two"
	docker run --name=two-auth \
		--hostname two-auth \
		--ip 172.10.1.2 \
		--volume $(THISDIR)/data/two/auth:/var/lib/teleport \
		$(DOCKEROPS) $(TELEBOX) build/teleport start -c /teleport/docker/two-auth.yaml
	docker run --name=two-proxy \
		--hostname two-proxy \
		--ip 172.10.1.3 \
		--publish 5080:5080 -p 5023:5023 \
		--volume $(THISDIR)/data/two/proxy:/var/lib/teleport \
		$(DOCKEROPS) $(TELEBOX) build/teleport start -c /teleport/docker/two-proxy.yaml
	docker run --name=two-node \
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
	-@docker network rm telenet

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
