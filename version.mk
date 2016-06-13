VERSION=0.9.0
SUFFIX=stable
GITTAG=v$(VERSION)-$(SUFFIX)
GITREF=$(shell git describe --dirty --tags)

FOO="\n\
one\ntwo"

.PHONY:all
all:
	@echo $(GITTAG)
	@echo $(GITREF)
