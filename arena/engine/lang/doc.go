// Package lang implements BOTSCRIPT, the tiny JavaScript subset bots are
// written in (see LANGUAGE.md). This file set is the front end: parse with
// goja/parser, then walk the AST against the section 8 whitelist. The result
// is a Program that the interpreter consumes. The package is deterministic:
// no wall clock, no global rand, error lists are sorted.
package lang
