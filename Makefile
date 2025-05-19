.DEFAULT_GOAL = check

# renovate: datasource=github-releases depName=golangci/golangci-lint
GOLANGCI_VERSION ?= v2.1.6
TEST_ARGS=-v -timeout 40s

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

.PHONY: patch_vendor
patch-vendor: go-mod-vendor
	./scripts/patch_vendor.sh

bin/vlt: patch-vendor go-mod-tidy
	go build -o "bin/vlt" ./cmd/vlt

bin/vltd: go-mod-tidy
	go build -o "bin/vltd" ./cmd/vltd

.PHONY: build
build: bin/vlt bin/vltd

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
	rm -rf bin/ coverage/

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