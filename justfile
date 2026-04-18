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

# Build from source and run the installer directly. Exit code 10 is
# the installer's "shell reload requested" signal, not a failure.
run: build
    #!/usr/bin/env bash
    set -u
    ./installer/dotsetup
    rc=$?
    if [[ $rc -eq 10 ]]; then exit 0; else exit $rc; fi

# Remove built binary
clean:
    rm -f installer/dotsetup

# Cut a new release. Bumps the latest v-tag and pushes it, which
# triggers .github/workflows/release-installer.yml to build, sign,
# and publish binaries. Requires a clean main branch.
#
#   just release            # patch  (v0.0.31 → v0.0.32)
#   just release minor      # minor  (v0.0.31 → v0.1.0)
#   just release major      # major  (v0.0.31 → v1.0.0)
#   just release v1.2.3     # explicit tag
release BUMP='patch':
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ -n "$(git status --porcelain)" ]]; then
        echo "✗ working tree is dirty — commit or stash first" >&2
        exit 1
    fi
    branch="$(git rev-parse --abbrev-ref HEAD)"
    if [[ "$branch" != "main" ]]; then
        echo "✗ refusing to release from '$branch' (expected main)" >&2
        exit 1
    fi
    git fetch --tags --quiet
    latest="$(git tag --list 'v*' --sort=-v:refname | head -n1)"
    latest="${latest:-v0.0.0}"
    case "{{BUMP}}" in
        v[0-9]*.[0-9]*.[0-9]*)
            next="{{BUMP}}" ;;
        major|minor|patch)
            IFS='.' read -r maj min pat <<< "${latest#v}"
            case "{{BUMP}}" in
                major) next="v$((maj+1)).0.0" ;;
                minor) next="v${maj}.$((min+1)).0" ;;
                patch) next="v${maj}.${min}.$((pat+1))" ;;
            esac ;;
        *)
            echo "✗ unknown bump/version: {{BUMP}}" >&2
            exit 1 ;;
    esac
    if git rev-parse --verify --quiet "refs/tags/$next" >/dev/null; then
        echo "✗ tag $next already exists" >&2
        exit 1
    fi
    echo "previous: $latest"
    echo "next:     $next"
    read -r -p "tag and push $next? [y/N] " reply
    [[ "$reply" =~ ^[Yy]$ ]] || { echo "aborted"; exit 1; }
    git tag -a "$next" -m "Release $next"
    # Push main first so branch-followers (other machines running
    # `git pull --ff-only`) pick up the commits that went into the
    # tag. Pushing only the tag ships CI's release artifact but
    # leaves origin/main frozen — see dock/v0.1.2 regression.
    git push origin main && git push origin "$next"
    echo "✓ pushed main + $next — release workflow will build & publish"
