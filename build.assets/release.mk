# This file is distributed with every binary Teleport tarball
BINDIR=/usr/local/bin
VARDIR=/var/lib/teleport

#
# sudo make install: installs Teleport into a UNIX-like OS
#
.PHONY: install
install: sudo
	mkdir -p $(VARDIR) $(BINDIR)
	cp -f teleport tctl tsh $(BINDIR)/

#
# sudo make uninstall: removes Teleport
#
.PHONY: uninstall
uninstall: sudo
	rm -rf $(VARDIR) $(BINDIR)/tctl $(BINDIR)/teleport $(BINDIR)/tsh


# helper: makes sure it runs as root
.PHONY:sudo
sudo:
	@if [ $$(id -u) != "0" ]; then \
		echo "ERROR: You must be root" && exit 1 ;\
	fi
