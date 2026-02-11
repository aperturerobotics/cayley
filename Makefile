SHELL:=bash
APTRE := go run -mod=mod github.com/aperturerobotics/common/cmd/aptre

all:

vendor:
	go mod vendor

generate gen genproto:
	$(APTRE) generate

clean:
	$(APTRE) clean

deps protodeps:
	$(APTRE) deps

lint:
	$(APTRE) lint

fix:
	$(APTRE) fix

test:
	$(APTRE) test

format fmt gofumpt:
	$(APTRE) format

goimports:
	$(APTRE) goimports

outdated:
	$(APTRE) outdated

release:
	$(APTRE) release

.PHONY: all vendor generate gen genproto clean deps protodeps lint fix test format fmt gofumpt goimports outdated release
