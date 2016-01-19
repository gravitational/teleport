#
# This Makefile is used for CI/CD builds. Please be careful!
#
BBOX=teleport-buildbox:latest
OUT="../"

#
# builds 'teleport' binary and places it $(OUT) dir
#
build: bbox
	docker run -i --rm=true \
		   -v $$(pwd)/../:/gopath/src/github.com/gravitational/teleport \
		   -u $$(id -u):$$(id -g) \
		   $(BBOX) \
		   /bin/bash -c "make -C /gopath/src/github.com/gravitational/teleport all"
	@echo "\nSUCCESS ----> $(OUT)teleport"
	@echo "SUCCESS ----> $(OUT)tctl"
	@echo "SUCCESS ----> $(OUT)tsh"

#
# builds buildbox:1.0.0 Docker container which is Debian8 with Golang 1.4.2
#
bbox:
	docker build --tag $(BBOX) .
