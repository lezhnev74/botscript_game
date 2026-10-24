package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// getenv looks up one setting by name; empty means unset.
type getenv func(name string) string

// loadEnv reads the dotenv file at path and returns a lookup where a variable
// set in the process environment wins over the file. A missing file is fine.
// Lines are KEY=VALUE, optionally prefixed with "export "; blank lines and
// lines starting with # are skipped; a value wrapped in matching ' or " quotes
// is unwrapped.
func loadEnv(path string, lookup func(string) (string, bool)) (getenv, error) {
	file, err := readEnvFile(path)
	if err != nil {
		return nil, err
	}
	return func(name string) string {
		if v, ok := lookup(name); ok {
			return v
		}
		return file[name]
	}, nil
}

func readEnvFile(path string) (map[string]string, error) {
	vars := map[string]string{}
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return vars, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		key, val, ok, err := parseEnvLine(sc.Text())
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		if ok {
			vars[key] = val
		}
	}
	return vars, sc.Err()
}

// parseEnvLine splits one dotenv line; ok is false for blank and comment lines.
func parseEnvLine(line string) (key, val string, ok bool, err error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false, nil
	}
	key, val, found := strings.Cut(strings.TrimPrefix(line, "export "), "=")
	key = strings.TrimSpace(key)
	if !found || key == "" {
		return "", "", false, errors.New("want KEY=VALUE")
	}
	return key, unquote(strings.TrimSpace(val)), true, nil
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}
