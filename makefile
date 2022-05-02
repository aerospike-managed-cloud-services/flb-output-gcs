#!/usr/bin/env make

SHELL 	:= /usr/bin/env bash
TARGET  := out_gcs.so
VERSION := $(shell tools/describe-version)
GOOS 	:= $(shell go env GOOS)
GOARCH 	:= $(shell go env GOARCH)
TARBALL := flb-output-gcs-$(VERSION)_$(GOOS)_$(GOARCH).tar.gz
BATS    := $(shell npm bin)/bats
FB_BIN  := $(shell which fluent-bit)
FB_OUTPUT_NAME := gcs
# increase this number as coverage improves
COVERAGE_PCT := 88

-include .env

.PHONY: clean deps-test print-release-artifact tarball test test-simple test-88pct

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
	# go install with an exact @version ignores go.mod
	go install github.com/dave/courtney@v0.3.1

test: deps-test
	courtney .
	go tool cover -func coverage.out

test-html-coverage: deps-test
	courtney .
	go tool cover -html coverage.out -o coverage.html

# check that coverage is at least XX%
test-coverage-enforced: test
	pct=`go tool cover -func coverage.out | grep '^total:' | grep -Po '\d+(?=\.)'`; \
	[[ $$pct -ge $(COVERAGE_PCT) ]] || false
