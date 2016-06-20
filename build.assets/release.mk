# This file is distributed with every binary Teleport tarball
BINDIR=/usr/local/bin
VARDIR=/var/lib/teleport
WEBDIR=/usr/local/share/teleport

#
# sudo make install: installs Teleport into a UNIX-like OS
#
.PHONY: install
install: sudo
	mkdir -p $(VARDIR) $(WEBDIR) $(BINDIR)
	cp -f teleport tctl tsh $(BINDIR)/
	cp -fr app index.html $(WEBDIR)/

#
# sudo make uninstall: removes Teleport
#
.PHONY: uninstall
uninstall: sudo
	rm -rf $(VARDIR) $(WEBDIR) $(BINDIR)/tctl $(BINDIR)/teleport $(BINDIR)/tsh


# helper: makes sure it runs as root
.PHONY:sudo
sudo:
	@if [ $$(id -u) != "0" ]; then \
		echo "ERROR: You must be root" && exit 1 ;\
	fi
