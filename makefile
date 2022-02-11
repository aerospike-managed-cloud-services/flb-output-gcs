#!/usr/bin/env make

SHELL 	:= /usr/bin/bash
TARGET  := out_gcs.so
VERSION := $(shell tools/describe-version)
GOOS 	:= $(shell go env GOOS)
GOARCH 	:= $(shell go env GOARCH)
TARBALL := flb-output-gcs-$(VERSION)_$(GOOS)_$(GOARCH).tar.gz
BATS    := $(shell npm bin)/bats
FB_BIN  := $(shell which fluent-bit)
FB_OUTPUT_NAME := gcs

.PHONY: clean test tarball

all: $(TARGET)

$(TARGET): *.go go.mod go.sum
	go build -buildmode=c-shared -o $@ --ldflags="-X main.VERSION=$(VERSION)"

$(TARBALL): $(TARGET)
	tar cfz $@ $^ && tar tvfz $@

tarball:
	$(MAKE) $(TARBALL)
	@echo "::set-output name=release_artifact::$(TARBALL)"

clean:
	rm -f $(TARGET) $(TARBALL)

test:
	go test

## test-bats: $(TARGET)
## 	cd test; $(BATS) test.bats

test-simple: $(TARGET)
	$(FB_BIN) -e ./$(TARGET) -c test/fluent-bit.conf 2>&1
