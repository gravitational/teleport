IMAGENAME=teleport:latest

IP_AUTH_A  = 172.10.0.10
IP_PROXY_A = 172.10.0.11
IP_NODE_A  = 172.10.0.12

IP_AUTH_B  = 172.10.0.20
IP_PROXY_B = 172.10.0.21
IP_NODE_B  = 172.10.0.22

DOCKEROPTS=--net telenet \
		-P \
		-u $$(id -u) \
		-v "$$(pwd)/../build":/opt/teleport \
		-v /lib:/lib \
		-v /lib64:/lib64

.PHONY: auth
auth: build network
	docker run --detach $(DOCKEROPTS) -h $(IP_AUTH_A) --ip $(IP_AUTH_A) --name tele-auth-a \
		-v $$(pwd)/cluster-a/auth:/var/lib/teleport \
		$(IMAGENAME) /opt/teleport/teleport start --roles=auth -d
	docker run --detach $(DOCKEROPTS) -h $(IP_AUTH_B) --ip $(IP_AUTH_B) --name tele-auth-b \
		-v $$(pwd)/cluster-b/auth:/var/lib/teleport \
		$(IMAGENAME) /opt/teleport/teleport start --roles=auth -d


.PHONY: proxies
proxies: build 
	docker rm -f tele-proxy-a || true
	docker rm -f tele-proxy-b || true
	docker run -ti $(DOCKEROPTS) -h $(IP_PROXY_A) --ip $(IP_PROXY_A) --name tele-proxy-a \
		-v $$(pwd)/cluster-a/proxy:/var/lib/teleport \
		$(IMAGENAME) /opt/teleport/teleport start --auth-server=$(IP_AUTH_A) --roles=proxy -d
#	docker run --detach $(DOCKEROPTS) -h $(IP_PROXY_B) --ip $(IP_PROXY_B) --name tele-proxy-b \
#		-v $$(pwd)/cluster-b/proxy:/var/lib/teleport \
#		$(IMAGENAME) /opt/teleport/teleport start --auth-server=$(IP_AUTH_B) --roles=proxy -d

.PHONY: nodes
nodes: build
	docker rm -f tele-node-a || true
	docker rm -f tele-node-b || true
	docker run --detach $(DOCKEROPTS) -h $(IP_NODE_A) --ip $(IP_NODE_A) --name tele-node-a \
		-v $$(pwd)/cluster-a/node:/var/lib/teleport \
		$(IMAGENAME) \
		/opt/teleport/teleport start --roles=node -d --token=n7fa9a32893c4ad384865700108a21256 --auth-server=$(IP_AUTH_A):3025
	docker run --detach $(DOCKEROPTS) -h $(IP_NODE_B) --ip $(IP_NODE_B) --name tele-node-b \
		-v $$(pwd)/cluster-b/node:/var/lib/teleport \
		$(IMAGENAME) \
		/opt/teleport/teleport start --roles=node -d --token=n9e53d9c9ad5501881e0e4c23bfaf01f8 --auth-server=$(IP_AUTH_B):3025

.PHONY: network
network: kill
	docker network rm telenet || true
	docker network create --subnet=172.10.0.0/16 telenet

.PHONY: build
build:
	docker build --build-arg UID=$$(id -u) --build-arg UNAME=$$(id -un) --tag $(IMAGENAME) .

.PHONY: clean
clean: kill
	rm -rf cluster-*
	mkdir -p cluster-a/auth
	mkdir -p cluster-a/proxy
	mkdir -p cluster-a/node
	mkdir -p cluster-b/auth
	mkdir -p cluster-b/proxy
	mkdir -p cluster-b/node

.PHONY: kill
kill:
	docker rm -f tele-auth-a || true
	docker rm -f tele-auth-b || true
	docker rm -f tele-proxy-a || true
	docker rm -f tele-proxy-b || true
	docker rm -f tele-node-a || true
	docker rm -f tele-node-b || true
