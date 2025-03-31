.DEFAULT_GOAL = check

# renovate: datasource=github-releases depName=golangci/golangci-lint
GOLANGCI_VERSION ?= v2.0.2
TEST_ARGS=-v -timeout 40s

bin/golangci-lint-${GOLANGCI_VERSION}:
	@mkdir -p bin
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
    	| sh -s -- -b ./bin  $(GOLANGCI_VERSION)
	@mv bin/golangci-lint "$@"

bin/golangci-lint: bin/golangci-lint-${GOLANGCI_VERSION}
	@ln -sf golangci-lint-${GOLANGCI_VERSION} bin/golangci-lint

bin/vlt: go-mod-tidy
	go build -o "bin/vlt" ./cmd/vlt

.PHONY: go-mod-tidy
go-mod-tidy:
	go mod tidy

.PHONY: clean
clean:
	go clean -testcache
	rm -rf bin/ coverage/

.PHONY: test
test:
	go test $(TEST_ARGS) ./...

.PHONY: cover
cover:
	@mkdir -p coverage
	go test $(TEST_ARGS) ./... -coverprofile coverage/cover.out

.PHONY: coverage-html
coverage-html: cover
	go tool cover -html=coverage/cover.out -o coverage/index.html

.PHONY: lint
lint: bin/golangci-lint
	bin/golangci-lint run

.PHONY: fix
fix: bin/golangci-lint
	bin/golangci-lint run --fix

.PHONY: check
check: lint test