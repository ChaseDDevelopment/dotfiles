package platform

// This file intentionally leaves detectPackageManager branch
// coverage to TestDetectPackageManagerWithSeams in
// detect_seam_test.go. That test drives every manager branch via
// the hasCommandFn seam so a typo in the binary-name string literals
// (e.g. "apt-get" → "apt get") is caught without re-implementing the
// if-ladder here.
