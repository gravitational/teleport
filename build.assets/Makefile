#
# This Makefile is used for CI/CD builds. Please be careful!
#
BBOX=teleport-buildbox:latest
OUT=$(realpath ../)
TELEPORT_PACKAGE=$(OUT)/teleport-app.tar.gz

# Build artifacts
BUILD_BUCKET_URL=s3://builds.gravitational.io/teleport
BUILD_TAG=$(shell git describe)

all: build build-package

#
# builds 'teleport' binary and places it $(OUT) dir
#
build: bbox
	docker run -i --rm=true \
		   -v $$(pwd)/../:/gopath/src/github.com/gravitational/teleport \
		   -u $$(id -u):$$(id -g) \
		   $(BBOX) \
		   /bin/bash -c "make -C /gopath/src/github.com/gravitational/teleport FLAGS='-cover -race' clean test all"
	@echo "\nSUCCESS ----> $(OUT)teleport"
	@echo "SUCCESS ----> $(OUT)tctl"
	@echo "SUCCESS ----> $(OUT)tsh"

#
# builds buildbox:1.0.0 Docker container which is Debian8 with Golang 1.4.2
#
bbox:
	docker build --tag $(BBOX) .


#
# starts teleport inside a build container
#
run: build
	docker run -i --rm=true \
		   -v $$(pwd)/../:/gopath/src/github.com/gravitational/teleport \
		   -u $$(id -u):$$(id -g) \
		   $(BBOX) \
		   /gopath/src/github.com/gravitational/teleport/out/teleport start -d

build-package: TMPDIR := $(shell mktemp -d)
build-package: $(TELEPORT_PACKAGE)
$(TELEPORT_PACKAGE):
	cp $(OUT)/assets/build/orbit.manifest.json $(TMPDIR)
	mkdir -p $(TMPDIR)/rootfs/usr/bin
	mkdir -p $(TMPDIR)/rootfs/usr/bin $(TMPDIR)/rootfs/etc/web-assets/
	cp -r $(OUT)/assets/web/* $(TMPDIR)/rootfs/etc/web-assets/
	cp -f $(OUT)/teleport $(TMPDIR)/rootfs/usr/bin
	cp -f $(OUT)/tctl $(TMPDIR)/rootfs/usr/bin
	cp -f $(OUT)/tsh $(TMPDIR)/rootfs/usr/bin
	tar -C $(TMPDIR) -czf $@ .
	rm -rf $(TMPDIR)

#
# deploys build artifacts to Amazon S3
#
deploy: build-package
	@echo "Deploying $(BUILD_TAG) to Amazon S3"
	aws s3 cp $(TELEPORT_PACKAGE) $(BUILD_BUCKET_URL)/$(BUILD_TAG)/
