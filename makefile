#!/usr/bin/env make

SHELL 			:= /usr/bin/env bash
TARGET  		:= out_gcs.so
TAGGED_VERSION	:= $(shell tools/describe-version)
#GOOS 			:= $(shell go env GOOS)
GOOS 			:= linux
#GOARCH 			:= $(shell go env GOARCH)
GOARCH 			:= amd64
TARBALL 		:= flb-output-gcs-$(TARGET_VERSION)_$(GOOS)_$(GOARCH).tar.gz
SOURCES			:= *.go go.mod go.sum
RELEASE_ARTIFACTS	:= $(TARBALL)
FB_BIN  		:= $(shell which fluent-bit)
# increase this number as coverage improves
COVERAGE_PCT	:= 88

-include .env

.PHONY: clean deps-test print-release-artifact tarball test test-simple

$(TARGET): $(SOURCES)
	go build -buildmode=c-shared -o $@ --ldflags="-X main.VERSION=$(TAGGED_VERSION)"

$(TARBALL): $(TARGET)
	tar cfz $@ $^ && tar tvfz $@

tarball:
	$(MAKE) $(TARBALL)

print-release-artifact:
	@echo $(RELEASE_ARTIFACTS)

clean:
	rm -f $(TARGET) $(TARBALL)

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
