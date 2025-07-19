.DEFAULT_GOAL = build

# vlt version
VERSION?=0.0.0

# renovate: datasource=github-releases depName=golangci/golangci-lint
GOLANGCI_VERSION ?= v2.2.2
TEST_ARGS=-v -timeout 40s

VLT_LDFLAGS= -X 'github.com/ladzaretti/vlt-cli/cli.Version=$(VERSION)'
VLTD_LDFLAGS= -X 'main.Version=$(VERSION)'

bin/golangci-lint-${GOLANGCI_VERSION}:
	@mkdir -p bin
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
    	| sh -s -- -b ./bin  $(GOLANGCI_VERSION)
	@mv bin/golangci-lint "$@"

bin/golangci-lint: bin/golangci-lint-${GOLANGCI_VERSION}
	@ln -sf golangci-lint-${GOLANGCI_VERSION} bin/golangci-lint


.PHONY: go-mod-vendor
go-mod-vendor:
	go mod vendor

.PHONY: patch-vendor
patch-vendor: go-mod-vendor
	./scripts/patch_vendor.sh

bin/vlt: patch-vendor go-mod-tidy
	go build -ldflags "$(VLT_LDFLAGS)" -o bin/vlt ./cmd/vlt

bin/vltd: go-mod-tidy
	go build -ldflags "$(VLTD_LDFLAGS)" -o bin/vltd ./cmd/vltd

.PHONY: build
build: bin/vlt bin/vltd

.PHONE: build-dist
build-dist: build
	mkdir -p dist
	cp ./bin/{vlt,vltd} UNLICENSE install.sh ./dist/
	cp -r ./systemd ./dist/

.PHONY: protoc
protoc:
	protoc \
		-I=./vaultdaemon/proto \
		-I=third_party \
		--go_out=./vaultdaemon/proto --go_opt=paths=source_relative \
		--go-grpc_out=./vaultdaemon/proto --go-grpc_opt=paths=source_relative \
		sessionpb/session.proto



.PHONY: go-mod-tidy
go-mod-tidy:
	go mod tidy

.PHONY: clean
clean:
	go clean -testcache
	rm -rf coverage/ bin/ dist/

.PHONY: test
test: patch-vendor
	go test $(TEST_ARGS) ./...

.PHONY: cover
cover: patch-vendor
	@mkdir -p coverage
	go test $(TEST_ARGS) ./... -coverprofile coverage/cover.out

.PHONY: coverage-html
coverage-html: cover
	go tool cover -html=coverage/cover.out -o coverage/index.html

.PHONY: lint
lint: bin/golangci-lint patch-vendor
	bin/golangci-lint run

.PHONY: fix
fix: bin/golangci-lint patch-vendor
	bin/golangci-lint run --fix

.PHONY: check
check: lint test

.PHONY: assets
assets: build
	./bin/vlt config generate > assets/default-config.toml
	./bin/vlt > assets/usage.txt

.PHONY: readme.md
readme.md: assets readme.templ.md assets/default-config.toml assets/usage.txt
	./scripts/readme_gen.sh readme.templ.md readme.md
	