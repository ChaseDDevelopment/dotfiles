package platform

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// OS represents the operating system type.
type OS int

const (
	MacOS OS = iota
	Linux
)

// String returns the human-readable OS name.
func (o OS) String() string {
	switch o {
	case MacOS:
		return "macOS"
	case Linux:
		return "Linux"
	default:
		return "Unknown"
	}
}

// Arch represents the CPU architecture.
type Arch int

const (
	AMD64 Arch = iota
	ARM64
)

// String returns the architecture string.
func (a Arch) String() string {
	switch a {
	case AMD64:
		return "x86_64"
	case ARM64:
		return "arm64"
	default:
		return "unknown"
	}
}

// PkgManagerType identifies which system package manager is available.
type PkgManagerType int

const (
	PkgBrew PkgManagerType = iota
	PkgApt
	PkgDnf
	PkgYum
	PkgPacman
	PkgZypper
	PkgNone
)

// String returns the package manager name.
func (p PkgManagerType) String() string {
	switch p {
	case PkgBrew:
		return "brew"
	case PkgApt:
		return "apt"
	case PkgDnf:
		return "dnf"
	case PkgYum:
		return "yum"
	case PkgPacman:
		return "pacman"
	case PkgZypper:
		return "zypper"
	default:
		return "none"
	}
}

// Platform holds detected information about the current system.
type Platform struct {
	OS             OS
	Arch           Arch
	OSName         string         // e.g., "macOS", "Ubuntu", "Arch Linux"
	OSVersion      string         // e.g., "15.4", "24.04"
	PackageManager PkgManagerType // detected package manager
	HasNala        bool           // apt systems: nala available as frontend
	HasYay         bool           // pacman systems: yay AUR helper available
	HasParu        bool           // pacman systems: paru AUR helper available
	// Warnings accumulates non-fatal detection issues so callers can
	// surface them in the TUI instead of having Detect() write to
	// stderr — stderr corrupts the alt-screen before the TUI owns
	// the terminal.
	Warnings []string
}

// Detect probes the current system and returns a Platform description.
func Detect() (*Platform, error) {
	p := &Platform{}

	switch runtime.GOOS {
	case "darwin":
		p.OS = MacOS
		p.OSName = "macOS"
		ver, err := macOSVersion()
		if err != nil {
			// Not fatal — most install strategies don't branch on
			// OSVersion — but collect the reason so distro-specific
			// mapping failures aren't mysterious. Buffered to avoid
			// corrupting the TUI alt-screen.
			p.Warnings = append(p.Warnings,
				fmt.Sprintf("detect macOS version: %v", err),
			)
		}
		p.OSVersion = ver
	case "linux":
		p.OS = Linux
		name, version, err := linuxDistro()
		if err != nil {
			p.Warnings = append(p.Warnings,
				fmt.Sprintf("detect linux distro: %v", err),
			)
		}
		p.OSName = name
		p.OSVersion = version
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	switch runtime.GOARCH {
	case "amd64":
		p.Arch = AMD64
	case "arm64":
		p.Arch = ARM64
	default:
		return nil, fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	p.PackageManager = detectPackageManager()
	p.HasNala = HasCommand("nala")
	p.HasYay = HasCommand("yay")
	p.HasParu = HasCommand("paru")

	return p, nil
}

// HasCommand checks whether a command is available in PATH.
func HasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// IsDesktopEnvironment returns true if a graphical display is available.
func (p *Platform) IsDesktopEnvironment() bool {
	if p.OS == MacOS {
		return true
	}
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

func macOSVersion() (string, error) {
	out, err := exec.Command("sw_vers", "-productVersion").Output()
	if err != nil {
		return "", fmt.Errorf("sw_vers: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func linuxDistro() (name, version string, err error) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "Linux", "", fmt.Errorf("read /etc/os-release: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "NAME=") {
			name = strings.Trim(strings.TrimPrefix(line, "NAME="), "\"")
		}
		if strings.HasPrefix(line, "VERSION_ID=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
		}
	}
	if name == "" {
		name = "Linux"
		return name, version, fmt.Errorf(
			"/etc/os-release has no NAME= line",
		)
	}
	return name, version, nil
}

func detectPackageManager() PkgManagerType {
	// Order matters: native system manager first on Linux so
	// linuxbrew users don't get routed through `brew install --cask`
	// (casks are macOS-only) when the tool registry offers a
	// homebrew strategy. Brew is only the preferred choice on
	// darwin.
	if runtime.GOOS == "darwin" && HasCommand("brew") {
		return PkgBrew
	}
	if HasCommand("apt-get") {
		return PkgApt
	}
	if HasCommand("dnf") {
		return PkgDnf
	}
	if HasCommand("yum") {
		return PkgYum
	}
	if HasCommand("pacman") {
		return PkgPacman
	}
	if HasCommand("zypper") {
		return PkgZypper
	}
	// Linux with brew (linuxbrew) as last resort — the registry's
	// apt/pacman/dnf strategies will be empty on systems without
	// those, so fall back to brew rather than erroring out.
	if HasCommand("brew") {
		return PkgBrew
	}
	return PkgNone
}
