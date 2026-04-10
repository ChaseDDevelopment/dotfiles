.PHONY: build test vet lint dev clean check

build:
	cd installer && go build -trimpath -o dotsetup .

test:
	cd installer && go test -race ./...

vet:
	cd installer && go vet ./...

lint:
	cd installer && golangci-lint run

dev: build
	./installer/dotsetup

clean:
	rm -f installer/dotsetup

check: vet test lint
