export TEXINPUTS := $(TEXINPUTS):$(abspath styles)

all: challenges/systems/worker.pdf challenges/fullstack/dashboard.pdf levels.pdf

%.pdf: %.tex
	cd $(@D) && pdflatex $(abspath $<)

.PHONY: install-ubuntu
install-ubuntu:
	sudo add-apt-repository ppa:jonathonf/texlive-2019
	sudo apt-get update
	sudo apt-get install texlive xzdec
	cd $(HOME) && tlmgr init-usertree
	tlmgr option repository ftp://tug.org/historic/systems/texlive/2017/tlnet-final
	tlmgr install listings csquotes

.PHONY: clean
clean:
	rm -f systems/worker.pdf challenges/fullstack/dashboard.pdf levels.pdf
