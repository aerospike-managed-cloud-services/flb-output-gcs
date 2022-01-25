#!/usr/bin/env make

SHELL 	:= /usr/bin/bash
PROG 	:= flb-output-gcs
VERSION := $(shell tools/describe-version)
TARBALL := $(PROG)-$(VERSION).tar.gz

.PHONY: clean test tarball


all: $(PROG)

$(PROG): *.go
	go build

$(TARBALL): $(PROG)
	tar cfz $@ $^ && tar tvfz $@

tarball: $(TARBALL)

clean:
	rm -f $(PROG) $(TARBALL)

test:
	go test
