# make mirror of Taskfile.yml for environments without task
export CGO_ENABLED := 1

BINARY := autochess.exe
# inner-loop passthrough: make test PKG=./internal/analytics RUN=-run=TestX
PKG ?= ./...
RUN ?=

.PHONY: build gen fmt lint vet test e2e playwright seed serve uiwatch dev check

build:
	go build -o $(BINARY) ./cmd/autochess

gen:
	go tool templ generate

fmt:
	gofmt -l -w .
	go tool templ fmt .

lint:
	golangci-lint run

vet:
	go vet $(PKG)

test:
	go test $(RUN) $(PKG)

e2e:
	go test -tags=e2e ./e2e/... -count=1 $(RUN)

playwright:
	go run ./e2e/cmd/playwright-install

seed:
	go run ./cmd/autochess -seed

serve:
	go run ./cmd/autochess

uiwatch:
	go tool templ generate --watch

# concurrent ui watch plus serve: run as make -j2 dev
dev: uiwatch serve

check: gen fmt lint vet test e2e
	git diff --exit-code -- internal/ui
