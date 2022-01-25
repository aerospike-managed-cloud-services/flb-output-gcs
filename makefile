#!/usr/bin/env make

SHELL 	:= /usr/bin/bash
PROG 	:= your-cool-program  ## (replace)
VERSION := $(shell tools/describe-version)
TARBALL := $(PROG)-$(VERSION).tar.gz

.PHONY: clean test tarball


all: $(PROG)

$(PROG): your.source.files  ## (replace)
	build-your-prog  ## (replace)

$(TARBALL): $(PROG)
	tar cfz $@ $^ && tar tvfz $@

tarball: $(TARBALL)

clean:
	rm -f $(PROG) $(TARBALL)

test:
	run-some-tests  ## (replace)
