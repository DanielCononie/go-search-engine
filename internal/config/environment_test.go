package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvironment(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(directory, ".env"),
		[]byte("# comment\n\nPLAIN=value\nQUOTED=\"two words\"\nSINGLE='literal value'\nexport EXPORTED=yes\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(workingDirectory)
	})
	t.Setenv("PLAIN", "")
	if err := os.Unsetenv("PLAIN"); err != nil {
		t.Fatal(err)
	}
	if err := os.Unsetenv("QUOTED"); err != nil {
		t.Fatal(err)
	}
	if err := os.Unsetenv("EXPORTED"); err != nil {
		t.Fatal(err)
	}
	if err := os.Unsetenv("SINGLE"); err != nil {
		t.Fatal(err)
	}

	if err := LoadEnvironment(); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("PLAIN") != "value" ||
		os.Getenv("QUOTED") != "two words" ||
		os.Getenv("SINGLE") != "literal value" ||
		os.Getenv("EXPORTED") != "yes" {
		t.Fatal("environment values were not loaded")
	}
}

func TestLoadEnvironmentDoesNotOverrideExistingValues(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(directory, ".env"),
		[]byte("EXISTING=file\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(workingDirectory)
	})
	t.Setenv("EXISTING", "process")

	if err := LoadEnvironment(); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("EXISTING") != "process" {
		t.Fatal("process environment was overridden")
	}
}

func TestLoadEnvironmentRejectsInvalidEntries(t *testing.T) {
	for _, value := range []string{
		"INVALID ENTRY\n",
		"9INVALID=value\n",
		"UNICODE_É=value\n",
		`UNTERMINATED="value`,
		`MISMATCHED='value"`,
	} {
		t.Run(value, func(t *testing.T) {
			directory := t.TempDir()
			if err := os.WriteFile(
				filepath.Join(directory, ".env"),
				[]byte(value),
				0o600,
			); err != nil {
				t.Fatal(err)
			}
			workingDirectory, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(directory); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = os.Chdir(workingDirectory)
			})

			if err := LoadEnvironment(); err == nil {
				t.Fatal("expected invalid environment entry error")
			}
		})
	}
}
