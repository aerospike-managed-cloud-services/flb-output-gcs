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

-include .env

.PHONY: clean deps-test print-release-artifact tarball test test-simple

all: $(TARGET)

$(TARGET): *.go go.mod go.sum
	go build -buildmode=c-shared -o $@ --ldflags="-X main.VERSION=$(VERSION)"

$(TARBALL): $(TARGET)
	tar cfz $@ $^ && tar tvfz $@

tarball:
	$(MAKE) $(TARBALL)

print-release-artifact:
	@echo "$(TARBALL)"

clean:
	rm -f $(TARGET) $(TARBALL)

## test-bats: $(TARGET)
## 	cd test; $(BATS) test.bats

test-simple: $(TARGET)
	OUT_GCS_DEV_LOGGING=yes $(FB_BIN) -e ./$(TARGET) -c test/fluent-bit.conf 2>&1

deps-test:
	go get -d github.com/dave/courtney
	go install github.com/dave/courtney

test:
	courtney .
	go tool cover -func coverage.out
	go tool cover -html coverage.out -o coverage.html

## test-100pct: deps-test
## 	courtney -e .
