// Command crap fails when any function has CRAP >= limit.
// CRAP(m) = comp^2 * (1-cov)^3 + comp, using a Go cover profile and go/ast complexity.
package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run returns 0 (ok), 1 (offenders found) or 2 (usage / input error).
func run(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("crap", flag.ContinueOnError)
	fl.SetOutput(stderr)
	profile := fl.String("profile", "coverage.out", "cover profile")
	root := fl.String("root", ".", "module root")
	limit := fl.Float64("max", 8, "fail when CRAP >= max")
	if fl.Parse(args) != nil {
		return 2
	}
	results, err := analyze(*root, *profile)
	if err != nil {
		fmt.Fprintln(stderr, "crap:", err)
		return 2
	}
	bad := offenders(results, *limit)
	for _, r := range bad {
		fmt.Fprintln(stdout, formatResult(r))
	}
	if len(bad) > 0 {
		return 1
	}
	return 0
}

func analyze(root, profilePath string) ([]result, error) {
	module, err := modulePath(root)
	if err != nil {
		return nil, err
	}
	byFile, err := loadProfile(profilePath, module)
	if err != nil {
		return nil, err
	}
	files, err := goFiles(root)
	if err != nil {
		return nil, err
	}
	var results []result
	for _, rel := range files {
		rs, err := analyzeOne(root, rel, byFile)
		if err != nil {
			return nil, err
		}
		results = append(results, rs...)
	}
	return results, nil
}

func analyzeOne(root, rel string, byFile map[string][]block) ([]result, error) {
	src, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return nil, err
	}
	infos, err := analyzeFile(rel, rel, src)
	if err != nil {
		return nil, err
	}
	var rs []result
	for _, fi := range infos {
		rs = append(rs, evaluate(fi, byFile))
	}
	return rs, nil
}

func loadProfile(path, module string) (map[string][]block, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	blocks, err := parseProfile(f)
	if err != nil {
		return nil, err
	}
	return groupByFile(blocks, module), nil
}

func modulePath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[0] == "module" {
			return f[1], nil
		}
	}
	return "", fmt.Errorf("no module line in %s/go.mod", root)
}

// goFiles lists non-test .go files (slash-separated, root-relative), skipping testdata, vendor and dot dirs.
func goFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			return skipDir(root, rel, d.Name())
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	return files, err
}

func skipDir(root, rel, name string) error {
	if rel == "." {
		return nil
	}
	if name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") {
		return filepath.SkipDir
	}
	if _, err := os.Stat(filepath.Join(root, rel, "go.mod")); err == nil {
		return filepath.SkipDir // nested module
	}
	return nil
}
