.PHONY: build test test-race vet lint fmt run examples clean install

build:
	go build -o kolang ./cmd/kolang

test:
	go test ./...

test-race:
	go test -race -count=1 ./...

vet:
	go vet ./...

lint:
	golangci-lint run

fmt:
	gofmt -w .

run:
	go run ./cmd/kolang

examples: build
	@for f in examples/*.kolang; do \
		./kolang "$$f" >/dev/null 2>&1 || { echo "FAIL: $$f"; exit 1; }; \
	done
	@echo "All examples passed"

clean:
	rm -f kolang

install:
	go install ./cmd/kolang