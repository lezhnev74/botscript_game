package main

import (
	"os"
	"path/filepath"
	"testing"
)

func noEnv(string) (string, bool) { return "", false }

func writeEnv(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadEnvReadsTheFile(t *testing.T) {
	path := writeEnv(t, "# comment\n\nA=1\nexport B = two words \nC=\"quoted # x\"\nD='single'\nE=\nF=a=b\nG=\"\n")
	get, err := loadEnv(path, noEnv)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"A": "1", "B": "two words", "C": "quoted # x", "D": "single", "E": "", "F": "a=b", "G": "\"", "H": ""}
	for k, v := range want {
		if got := get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestLoadEnvProcessEnvironmentWins(t *testing.T) {
	path := writeEnv(t, "A=file\nB=file\n")
	get, err := loadEnv(path, func(k string) (string, bool) {
		if k == "A" {
			return "", true // set but empty still overrides the file
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	if get("A") != "" || get("B") != "file" {
		t.Errorf("A=%q B=%q", get("A"), get("B"))
	}
}

func TestLoadEnvMissingFileIsFine(t *testing.T) {
	get, err := loadEnv(filepath.Join(t.TempDir(), "nope"), func(k string) (string, bool) { return "env", true })
	if err != nil {
		t.Fatal(err)
	}
	if get("X") != "env" {
		t.Error(get("X"))
	}
}

func TestLoadEnvRejectsBadLines(t *testing.T) {
	for _, text := range []string{"JUSTAKEY\n", "=value\n"} {
		if _, err := loadEnv(writeEnv(t, text), noEnv); err == nil {
			t.Errorf("%q accepted", text)
		}
	}
}

func TestLoadEnvUnreadableFileFails(t *testing.T) {
	if _, err := loadEnv(t.TempDir(), noEnv); err == nil { // a directory, not a file
		t.Error("want an error")
	}
}
