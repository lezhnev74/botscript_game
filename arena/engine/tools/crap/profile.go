package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// block is one line of a Go cover profile.
type block struct {
	File                string
	StartLine, StartCol int
	EndLine, EndCol     int
	Stmts, Count        int
}

func parseProfile(r io.Reader) ([]block, error) {
	var blocks []block
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "mode:") || line == "" {
			continue
		}
		b, err := parseBlock(line)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, sc.Err()
}

func parseBlock(line string) (block, error) {
	i := strings.LastIndex(line, ":")
	if i < 0 {
		return block{}, fmt.Errorf("bad cover line %q", line)
	}
	b := block{File: line[:i]}
	_, err := fmt.Sscanf(line[i+1:], "%d.%d,%d.%d %d %d",
		&b.StartLine, &b.StartCol, &b.EndLine, &b.EndCol, &b.Stmts, &b.Count)
	if err != nil {
		return block{}, fmt.Errorf("bad cover line %q: %w", line, err)
	}
	return b, nil
}

// groupByFile keys blocks by module-relative file path. Blocks with the same
// position (emitted once per test binary under -coverpkg) are merged keeping
// the maximum count, like go tool cover.
func groupByFile(blocks []block, module string) map[string][]block {
	type key struct {
		file                                 string
		startLine, startCol, endLine, endCol int
	}
	out := map[string][]block{}
	index := map[key]int{}
	for _, b := range blocks {
		file := strings.TrimPrefix(b.File, module+"/")
		k := key{file, b.StartLine, b.StartCol, b.EndLine, b.EndCol}
		if i, ok := index[k]; ok {
			if b.Count > out[file][i].Count {
				out[file][i].Count = b.Count
			}
			continue
		}
		index[k] = len(out[file])
		out[file] = append(out[file], b)
	}
	return out
}
