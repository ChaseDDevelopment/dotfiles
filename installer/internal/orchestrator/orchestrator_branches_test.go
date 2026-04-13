package orchestrator

import (
	"context"
	"testing"

	"github.com/chaseddevelopment/dotfiles/installer/internal/pkgmgr"
)

// TestDoctorTasksRunClosures runs every task's Run closure so the
// switch over CheckInstalled return values (NotInstalled, Outdated,
// Installed) is exercised. We don't assert specific errors — tools
// won't be installed in the test environment, so most closures will
// return "not installed" which is the dominant branch.
func TestDoctorTasksRunClosures(t *testing.T) {
	// Empty PATH so exec.LookPath fails for every tool → exercises
	// the NotInstalled branch of the doctor task closures.
	t.Setenv("PATH", "")
	bc := newTestBuildConfig(t)
	result := BuildDoctorTasks(bc)
	for _, task := range result.Tasks {
		if task.Run == nil {
			continue
		}
		_ = task.Run(context.Background())
	}
}

// TestInstallTasksRunClosuresDryRun builds the install task set with
// DryRun=true so each Run closure exercises the early-return branch
// for tool already-installed / dry-run paths, raising coverage on
// BuildInstallTasks's per-task closures without shelling out.
func TestInstallTasksRunClosuresDryRun(t *testing.T) {
	bc := newTestBuildConfig(t)
	bc.DryRun = true
	result := BuildInstallTasks(bc)
	for _, task := range result.Tasks {
		if task.Run == nil {
			continue
		}
		_ = task.Run(context.Background())
	}
}

// TestUninstallTasksRunClosures runs every uninstall Run closure so
// its filesystem-probe + remove branch (which skips when nothing is
// installed, the common test-env case) is counted.
func TestUninstallTasksRunClosures(t *testing.T) {
	bc := newTestBuildConfig(t)
	result := BuildUninstallTasks(bc)
	for _, task := range result.Tasks {
		if task.Run == nil {
			continue
		}
		_ = task.Run(context.Background())
	}
}

// TestBuildInstallTasksWithAptInjectsDpkgDoctor covers the Apt branch
// of BuildInstallTasks where a "dpkg-doctor" pseudo-task is prepended
// because the package manager is *pkgmgr.Apt.
func TestBuildInstallTasksWithAptInjectsDpkgDoctor(t *testing.T) {
	bc := newTestBuildConfig(t)
	bc.PkgMgr = pkgmgr.NewApt(bc.Runner, false)
	result := BuildInstallTasks(bc)
	found := false
	for _, task := range result.Tasks {
		if task.ID == "dpkg-doctor" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected dpkg-doctor pseudo-task for Apt manager")
	}
}

// TestBuildInstallTasksSkipPackagesOmitsDoctor covers the inverse:
// SkipPackages=true means no doctor task is added.
func TestBuildInstallTasksSkipPackagesOmitsDoctor(t *testing.T) {
	bc := newTestBuildConfig(t)
	bc.PkgMgr = pkgmgr.NewApt(bc.Runner, false)
	bc.SkipPackages = true
	result := BuildInstallTasks(bc)
	for _, task := range result.Tasks {
		if task.ID == "dpkg-doctor" {
			t.Fatal("dpkg-doctor task should be skipped when SkipPackages=true")
		}
	}
}

// TestUpdateTasksRunClosuresDryRun drives update closures with a
// dry-run runner so the task bodies execute without touching real
// commands.
func TestUpdateTasksRunClosuresDryRun(t *testing.T) {
	bc := newTestBuildConfig(t)
	bc.DryRun = true
	bc.Runner.DryRun = true
	result := BuildUpdateTasks(bc)
	for _, task := range result.Tasks {
		if task.Run == nil {
			continue
		}
		_ = task.Run(context.Background())
	}
}
