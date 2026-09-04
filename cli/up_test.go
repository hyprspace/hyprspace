package cli

import (
	"os"
	"testing"

	log "github.com/ipfs/go-log/v2"
)

func TestConfigureLogging(t *testing.T) {
	log.Logger("hyprspace/test")

	originalLevel, wasSet := os.LookupEnv("HYPRSPACE_LOG_LEVEL")
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv("HYPRSPACE_LOG_LEVEL", originalLevel)
		} else {
			_ = os.Unsetenv("HYPRSPACE_LOG_LEVEL")
		}
		if err := configureLogging(); err != nil {
			t.Errorf("restoring log level: %v", err)
		}
	})

	tests := []struct {
		name      string
		value     string
		expected  string
		wantError bool
	}{
		{name: "default", value: "", expected: "info"},
		{name: "debug", value: "debug", expected: "debug"},
		{name: "invalid", value: "invalid", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := os.Setenv("HYPRSPACE_LOG_LEVEL", test.value); err != nil {
				t.Fatal(err)
			}

			err := configureLogging()
			if test.wantError {
				if err == nil {
					t.Fatal("configureLogging() did not reject an invalid log level")
				}
				return
			}
			if err != nil {
				t.Fatalf("configureLogging(): %v", err)
			}

			for _, subsystem := range []string{"hyprspace", "hyprspace/test"} {
				level, err := log.SubsystemLevelName(subsystem)
				if err != nil {
					t.Fatalf("getting level for %s: %v", subsystem, err)
				}
				if level != test.expected {
					t.Errorf("level for %s = %q, want %q", subsystem, level, test.expected)
				}
			}
		})
	}
}
