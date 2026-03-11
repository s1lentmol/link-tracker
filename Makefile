COVERAGE_FILE ?= coverage.out

MODULES := bot scrapper

.PHONY: help
help:
	@echo "Available commands:"
	@echo "  \033[36mmake build\033[0m - Build all modules ($(MODULES))"
	@$(foreach mod,$(MODULES),echo "  \033[36mmake build_$(mod)\033[0m - Build $(mod) module";)
	@echo "  \033[36mmake test\033[0m - Run all tests"

.PHONY: build
build:
	@echo "Building all modules: $(MODULES)"
	@mkdir -p bin
	@go build -o ./bin/bot ./cmd/bot
	@go build -o ./bin/scrapper ./cmd/scrapper

.PHONY: build_bot
build_bot:
	@mkdir -p bin
	@go build -o ./bin/bot ./cmd/bot

.PHONY: build_scrapper
build_scrapper:
	@mkdir -p bin
	@go build -o ./bin/scrapper ./cmd/scrapper

.PHONY: test
test:
	@go test --race -count=1 -coverprofile='$(COVERAGE_FILE)' ./...
	@go tool cover -func='$(COVERAGE_FILE)' | grep ^total | tr -s '\t'
