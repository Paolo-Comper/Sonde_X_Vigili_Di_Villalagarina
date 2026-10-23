BINARY := argine

.PHONY: build test vet fmt run run-test dev clean

build:
	go build -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

run: build
	./$(BINARY) --no-browser

run-test: build
	./$(BINARY) --test --interval 2s

dev:
	go run . --dev --test

clean:
	rm -f $(BINARY) *.db *.db-shm *.db-wal
