#!/usr/bin/make -f

BIN := bakepkg
GO := go
PKGS := ./...

.PHONY: all
all: build check

.PHONY: build
build:
	$(GO) build -o $(BIN)

.PHONY: install
install:
	$(GO) install

.PHONY: check
check:
	$(GO) test -v -race $(PKGS)

.PHONY: lint
lint:
	golangci-lint run

.PHONY: fmt
fmt:
	$(GO) fmt $(PKGS)

.PHONY: mod-tidy
mod-tidy:
	$(GO) mod tidy

.PHONY: clean
clean:
	rm -f $(BIN)
	rm -f coverage.out

.PHONY: distclean
distclean: clean