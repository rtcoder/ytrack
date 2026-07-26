package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergePrefersLocalValuesAndKeepsProjectLocalOnly(t *testing.T) {
	global := Config{
		URL:       "https://global.example",
		Token:     "global-token",
		ProjectID: "GLOBAL",
	}
	local := Config{
		URL:       "https://local.example",
		Token:     "local-token",
		ProjectID: "LOCAL",
	}

	got := Merge(global, local)

	if got.URL != "https://local.example" {
		t.Fatalf("URL = %q, want local override", got.URL)
	}
	if got.Token != "local-token" {
		t.Fatalf("Token = %q, want local override", got.Token)
	}
	if got.ProjectID != "LOCAL" {
		t.Fatalf("ProjectID = %q, want local project id", got.ProjectID)
	}
}

func TestMergeDoesNotUseGlobalProjectID(t *testing.T) {
	got := Merge(Config{ProjectID: "GLOBAL"}, Config{})

	if got.ProjectID != "" {
		t.Fatalf("ProjectID = %q, want empty because project_id is local-only", got.ProjectID)
	}
}

func TestSaveLoadAndUnsetConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ytrack", "config.json")

	if err := Save(path, Config{URL: "https://youtrack.example", Token: "perm:secret"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat saved config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("saved file mode = %o, want 0600", got)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.URL != "https://youtrack.example" || got.Token != "perm:secret" {
		t.Fatalf("Load() = %+v, want saved values", got)
	}
}

func TestMaskTokenNeverRevealsFullToken(t *testing.T) {
	got := MaskToken("perm:abcdefghijklmnopqrstuvwxyz")

	if got != "perm:xxxx...wxyz" {
		t.Fatalf("MaskToken() = %q, want prefix plus masked body and last four chars", got)
	}
}

func TestRequireEffectiveFieldsReturnsActionableErrors(t *testing.T) {
	effective := Config{URL: "https://youtrack.example"}

	err := Require(effective, "url", "token", "project_id")
	if err == nil {
		t.Fatal("Require() error = nil, want missing token/project_id error")
	}
	want := "missing configured token, run `ytrack set-token <token>` or `ytrack global set-token <token>`"
	if err.Error() != want {
		t.Fatalf("Require() error = %q, want %q", err.Error(), want)
	}
}
