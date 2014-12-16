DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)

all: deps format
	@go build -o delancey.out
	@./delancey.out ${ARGS}

deps:
	@go get -d -v ./...
	@echo $(DEPS) | xargs -n1 go get -d

format:
	@gofmt -w .

test:
	@go test ./...

release:
	@bash --norc ./scripts/release_agent_simple.sh

agent: release

clean:
	-rm -rf delancey.out pkg debug.log

.PHONY: all deps format test release agent clean
