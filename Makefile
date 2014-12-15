DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)

all: deps format
	@go build -o delancey.out
	@./delancey.out --docker ${DOCKER_ADDR}

deps:
	@go get -d -v ./...
	@echo $(DEPS) | xargs -n1 go get -d

format:
	@gofmt -w .

test:
	@go test ./...

release:
	@bash --norc ./scripts/release_agent.sh

agent: release

clean:
	-rm -rf delancey.out pkg debug.log

.PHONY: all deps format test release agent clean
