// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0
// https://www.apache.org/licenses/LICENSE-2.0

package version

import (
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	t.Parallel()
	if Version == "" {
		t.Error("Version should not be empty")
	}

	// Version should be semver-like
	parts := strings.Split(Version, ".")
	if len(parts) < 2 {
		t.Errorf("Version should have at least major.minor format, got: %s", Version)
	}
}
