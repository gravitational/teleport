IMAGENAME=teleport:latest
DOCKEROPTS=--net telenet \
		-P \
		-u $$(id -u) \
		-v "$$(pwd)/../build":/opt/teleport \
		-v /lib:/lib \
		-v /lib64:/lib64 \
		-v $$(pwd)/data:/var/lib/teleport

.PHONY: run-auth
run-auth: build network
	docker run -ti --rm=true $(DOCKEROPTS) --ip 172.10.0.10 \
		$(IMAGENAME) /opt/teleport/teleport start --roles=auth -d

.PHONY: network
network:
	docker network rm telenet || /bin/true
	docker network create --subnet=172.10.0.0/16 telenet

.PHONY: build
build:
	docker build --tag $(IMAGENAME) .
