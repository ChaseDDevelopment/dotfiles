# dotfiles — common dev tasks. Run `just` with no args to list recipes.

default:
    @just --list

# Build the Go TUI installer
build:
    cd installer && go build -o dotsetup .

# Static analysis
vet:
    cd installer && go vet ./...

# Run the Go test suite
test:
    cd installer && go test ./...

# Full local check: vet + test
check: vet test

# Run the TUI installer (downloads binary if missing, then launches)
install:
    ./install.sh

# Build from source and run the installer directly
run: build
    ./installer/dotsetup

# Remove built binary
clean:
    rm -f installer/dotsetup
