COVERAGE_FILE ?= coverage.out

# Get all directories in cmd/ as available modules and add moved services
MODULES := $(sort $(notdir $(wildcard cmd/*)) bot)

# Help target - display usage information
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
	@$(foreach mod,$(MODULES),echo "Building module: $(mod)"; if [ "$(mod)" = "bot" ]; then go build -o ./bin/$(mod) ./services/bot/cmd/$(mod); else go build -o ./bin/$(mod) ./cmd/$(mod); fi;)

# Convenience targets for building individual modules
.PHONY: $(addprefix build_,$(MODULES))
$(addprefix build_,$(MODULES)):
	@modulename=$(subst build_,,$@); \
	echo "Building module: $$modulename"; \
	mkdir -p bin; \
	if [ "$$modulename" = "bot" ]; then \
		go build -o ./bin/$$modulename ./services/bot/cmd/$$modulename; \
	else \
		go build -o ./bin/$$modulename ./cmd/$$modulename; \
	fi

## test: run all tests
.PHONY: test
test:
	@go test --race -count=1 -coverprofile='$(COVERAGE_FILE)' ./...
	@go tool cover -func='$(COVERAGE_FILE)' | grep ^total | tr -s '\t'
