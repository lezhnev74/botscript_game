package lang

import (
	"fmt"
	"sort"
)

// graph checks recursion (direct and mutual) and call depth on the edges
// collected while walking the bodies.
func (c *checker) graph() {
	reach := c.reachability()
	acyclic := c.edges[:0:0]
	for _, e := range c.edges {
		if e.from == e.to || reach[e.to][e.from] {
			c.recursion(e)
			continue
		}
		acyclic = append(acyclic, e)
	}
	c.depth(acyclic)
}

func (c *checker) recursion(e edge) {
	if e.from == e.to {
		c.errAt(e.at, fmt.Sprintf("recursion is not allowed: '%s' calls itself", e.from),
			"a function cannot call itself (the stack would be hidden memory); use a while loop and num[0] as counter")
		return
	}
	c.errAt(e.at, fmt.Sprintf("recursion is not allowed: '%s' calls '%s', which leads back to '%s'", e.from, e.to, e.from),
		fmt.Sprintf("'%s' and '%s' call each other in a circle; use a while loop and num[0..7] instead", e.from, e.to))
}

// reachability[a][b] is true when a can (transitively) call b.
func (c *checker) reachability() map[string]map[string]bool {
	next := map[string][]string{}
	for _, e := range c.edges {
		next[e.from] = append(next[e.from], e.to)
	}
	out := map[string]map[string]bool{}
	for _, f := range c.p.Order {
		out[f.Name] = walkFrom(f.Name, next)
	}
	return out
}

func walkFrom(start string, next map[string][]string) map[string]bool {
	seen := map[string]bool{}
	stack := append([]string(nil), next[start]...)
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if !seen[n] {
			seen[n] = true
			stack = append(stack, next[n]...)
		}
	}
	return seen
}

// depth computes the longest call chain from any entry point over the
// acyclic edges and reports every call that would exceed MaxCallDepth.
func (c *checker) depth(edges []edge) {
	d := c.entryDepths()
	for range c.p.Order {
		relax(d, edges)
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].at < edges[j].at })
	for _, e := range edges {
		if d[e.from] > 0 && d[e.from]+1 > MaxCallDepth {
			c.tooDeep(e, d[e.from]+1)
		}
	}
}

func (c *checker) entryDepths() map[string]int {
	d := map[string]int{}
	for _, root := range []*Func{c.p.Loop, c.p.OnHit, c.p.OnBump} {
		if root != nil {
			d[root.Name] = 1
		}
	}
	return d
}

func relax(d map[string]int, edges []edge) {
	for _, e := range edges {
		if d[e.from] > 0 && d[e.from]+1 > d[e.to] {
			d[e.to] = d[e.from] + 1
		}
	}
}

func (c *checker) tooDeep(e edge, depth int) {
	c.errAt(e.at, fmt.Sprintf("call depth would be %d (max %d): '%s' -> '%s'", depth, MaxCallDepth, e.from, e.to),
		fmt.Sprintf("merge '%s' into its caller, or call it from a function closer to loop()", e.to))
}
