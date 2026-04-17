package tui

import (
	"testing"
)

// TestShellReloadArmsOnInstall covers the happy path: after a
// successful install run, main.go should see ShellReloadPending()
// return true so install.sh can exec a fresh shell.
func TestShellReloadArmsOnInstall(t *testing.T) {
	cfg := newTestConfig()
	cfg.Mode = ModeInstall
	app := NewApp(cfg)

	if app.ShellReloadPending() {
		t.Fatal("reload should not be armed before install")
	}
	app.armShellReloadIfApplicable()
	if !app.ShellReloadPending() {
		t.Fatal("reload should be armed after successful install")
	}
}

// TestShellReloadArmsOnCustomInstall and Update — the other two
// modes that produce a fresh config state worth reloading into.
func TestShellReloadArmsOnCustomInstallAndUpdate(t *testing.T) {
	for _, mode := range []InstallMode{ModeCustomInstall, ModeUpdate} {
		cfg := newTestConfig()
		cfg.Mode = mode
		app := NewApp(cfg)
		app.armShellReloadIfApplicable()
		if !app.ShellReloadPending() {
			t.Errorf("mode %v: expected reload armed", mode)
		}
	}
}

// TestShellReloadSkippedForReadOnlyModes — doctor/restore/uninstall
// don't land the user in a state where dropping into a shiny new
// shell makes sense. Reload must stay disarmed.
func TestShellReloadSkippedForReadOnlyModes(t *testing.T) {
	for _, mode := range []InstallMode{
		ModeDoctor, ModeRestore, ModeUninstall, ModeDryRun,
	} {
		cfg := newTestConfig()
		cfg.Mode = mode
		app := NewApp(cfg)
		app.armShellReloadIfApplicable()
		if app.ShellReloadPending() {
			t.Errorf("mode %v: reload should not arm", mode)
		}
	}
}

// TestShellReloadSkippedOnCriticalFailure — no point dropping the
// user into a fresh shell when the install bailed mid-way. Let
// them stay in their current session to debug.
func TestShellReloadSkippedOnCriticalFailure(t *testing.T) {
	cfg := newTestConfig()
	cfg.Mode = ModeInstall
	app := NewApp(cfg)
	app.summary.criticalFailure = true
	app.armShellReloadIfApplicable()
	if app.ShellReloadPending() {
		t.Fatal("reload must not arm on critical failure")
	}
}

// TestShellReloadFlagSticksAcrossMenuNav — user's explicit request:
// if they re-enter the main menu after a successful install,
// running another action, or navigating around, the flag stays on
// and a future quit still reloads.
func TestShellReloadFlagSticksAcrossMenuNav(t *testing.T) {
	cfg := newTestConfig()
	cfg.Mode = ModeInstall
	app := NewApp(cfg)
	app.armShellReloadIfApplicable()

	// Simulate going back to main menu → choosing doctor →
	// completing that run. Doctor alone wouldn't arm the flag,
	// but the prior install should have already armed it and
	// re-arming logic must not clear it.
	app.phase = PhaseMainMenu
	app.config.Mode = ModeDoctor
	app.armShellReloadIfApplicable() // no-op for doctor

	if !app.ShellReloadPending() {
		t.Fatal("flag must persist through subsequent menu navigation")
	}
}
