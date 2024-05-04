package iterator

import (
	"context"
	"fmt"

	"github.com/cayleygraph/cayley/graph/refs"
	"github.com/cayleygraph/cayley/quad"
)

// Chain is a chain-enabled helper to setup iterator execution.
type Chain struct {
	s  Shape
	it Scanner
	qs refs.Namer

	paths    bool
	optimize bool

	limit int
	n     int
}

// Iterate is a set of helpers for iteration. Context may be used to cancel execution.
// Iterator will be optimized and closed after execution.
//
// By default, iteration has no limit and includes sub-paths.
func Iterate(it Shape) *Chain {
	return &Chain{
		s:     it,
		limit: -1, paths: true,
		optimize: true,
	}
}

func (c *Chain) next(ctx context.Context) bool {
	if err := ctx.Err(); err != nil {
		return false
	}
	ok := (c.limit < 0 || c.n < c.limit) && c.it.Next(ctx)
	if ok {
		c.n++
	}
	return ok
}

func (c *Chain) nextPath(ctx context.Context) bool {
	if err := ctx.Err(); err != nil {
		return false
	}
	ok := c.paths && (c.limit < 0 || c.n < c.limit) && c.it.NextPath(ctx)
	if ok {
		c.n++
	}
	return ok
}

func (c *Chain) start(ctx context.Context) {
	if c.optimize {
		optim, _, err := c.s.Optimize(ctx)
		if err == nil {
			c.s = optim
		}
	}
	c.it = c.s.Iterate(ctx)
}

func (c *Chain) end() {
	c.it.Close()
}

// Limit limits a total number of results returned.
func (c *Chain) Limit(n int) *Chain {
	c.limit = n
	return c
}

// Paths switches iteration over sub-paths (with it.NextPath).
// Defaults to true.
func (c *Chain) Paths(enable bool) *Chain {
	c.paths = enable
	return c
}

// On sets a default quad store for iteration. If qs was set, it may be omitted in other functions.
func (c *Chain) On(qs refs.Namer) *Chain {
	c.qs = qs
	return c
}

// UnOptimized disables iterator optimization.
func (c *Chain) UnOptimized() *Chain {
	c.optimize = false
	return c
}

// Each will run a provided callback for each result of the iterator.
func (c *Chain) Each(ctx context.Context, fnc func(refs.Ref) error) error {
	c.start(ctx)
	defer c.end()

	process := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		res, err := c.it.Result(ctx)
		if err != nil {
			return err
		}
		return fnc(res)
	}

	for c.next(ctx) {
		if err := process(); err != nil {
			return err
		}
		for c.nextPath(ctx) {
			if err := process(); err != nil {
				return err
			}
		}
	}
	return c.it.Err()
}

// Count returns the number of results in the Chain.
//
// In the best case it uses Stats to determine the count.
// In the worst case it will iterate over all values to count them.
func (c *Chain) Count(ctx context.Context) (int64, error) {
	// attempt to use Stats first.
	ch := c.s
	if c.optimize {
		optim, _, err := c.s.Optimize(ctx)
		if err == nil {
			ch = optim
		}
	}
	if st, err := ch.Stats(ctx); err != nil {
		return st.Size.Value, err
	} else if st.Size.Exact {
		return st.Size.Value, nil
	}

	// use iteration to count
	c.start(ctx)
	defer c.end()
	if err := c.it.Err(); err != nil {
		return 0, err
	}
	var cnt int64
	for c.next(ctx) {
		if err := ctx.Err(); err != nil {
			return cnt, err
		}
		cnt++
		for c.nextPath(ctx) {
			if err := ctx.Err(); err != nil {
				return cnt, err
			}
			cnt++
		}
	}
	return cnt, c.it.Err()
}

// All will return all results of an iterator.
func (c *Chain) All(ctx context.Context) ([]refs.Ref, error) {
	c.start(ctx)
	defer c.end()
	var out []refs.Ref

	process := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := c.it.Err(); err != nil {
			return err
		}
		res, err := c.it.Result(ctx)
		if err != nil {
			return err
		}
		out = append(out, res)
		return nil
	}

	for c.next(ctx) {
		if err := process(); err != nil {
			return out, err
		}
		for c.nextPath(ctx) {
			if err := process(); err != nil {
				return out, err
			}
		}
	}

	return out, c.it.Err()
}

// First will return a first result of an iterator. It returns nil if iterator is empty.
func (c *Chain) First(ctx context.Context) (refs.Ref, error) {
	c.start(ctx)
	defer c.end()
	if !c.next(ctx) {
		return nil, c.it.Err()
	}
	return c.it.Result(ctx)
}

// Send will send each result of the iterator to the provided channel.
//
// Channel will NOT be closed when function returns.
func (c *Chain) Send(ctx context.Context, out chan<- refs.Ref) error {
	c.start(ctx)
	defer c.end()
	done := ctx.Done()
	process := func() error {
		res, err := c.it.Result(ctx)
		if err != nil {
			return err
		}
		select {
		case <-done:
			return ctx.Err()
		case out <- res:
			return nil
		}
	}
	for c.next(ctx) {
		if err := process(); err != nil {
			return err
		}
		for c.nextPath(ctx) {
			if err := process(); err != nil {
				return err
			}
		}
	}
	return c.it.Err()
}

// TagEach will run a provided tag map callback for each result of the iterator.
func (c *Chain) TagEach(ctx context.Context, fnc func(map[string]refs.Ref) error) error {
	c.start(ctx)
	defer c.end()

	mn := 0
	process := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		tags := make(map[string]refs.Ref, mn)
		if err := c.it.TagResults(ctx, tags); err != nil {
			return err
		}
		if n := len(tags); n > mn {
			mn = n
		}
		return fnc(tags)
	}

	for c.next(ctx) {
		if err := process(); err != nil {
			return err
		}
		for c.nextPath(ctx) {
			if err := process(); err != nil {
				return err
			}
		}
	}

	return c.it.Err()
}

var errNoQuadStore = fmt.Errorf("no quad store in Iterate")

// EachValue is an analog of Each, but it will additionally call NameOf
// for each graph.Ref before passing it to a callback.
func (c *Chain) EachValue(ctx context.Context, qs refs.Namer, fnc func(quad.Value) error) error {
	if qs != nil {
		c.qs = qs
	}
	if c.qs == nil {
		return errNoQuadStore
	}
	// TODO(dennwc): batch NameOf?
	return c.Each(ctx, func(v refs.Ref) error {
		nv, err := c.qs.NameOf(ctx, v)
		if err == nil && nv != nil {
			err = fnc(nv)
		}
		return err
	})
}

// EachValuePair is an analog of Each, but it will additionally call NameOf
// for each graph.Ref before passing it to a callback. Original value will be passed as well.
func (c *Chain) EachValuePair(ctx context.Context, qs refs.Namer, fnc func(refs.Ref, quad.Value) error) error {
	if qs != nil {
		c.qs = qs
	}
	if c.qs == nil {
		return errNoQuadStore
	}
	// TODO(dennwc): batch NameOf?
	return c.Each(ctx, func(v refs.Ref) error {
		nv, err := c.qs.NameOf(ctx, v)
		if err == nil && nv != nil {
			err = fnc(v, nv)
		}
		return err
	})
}

// AllValues is an analog of All, but it will additionally call NameOf
// for each graph.Ref before returning the results slice.
func (c *Chain) AllValues(ctx context.Context, qs refs.Namer) ([]quad.Value, error) {
	var out []quad.Value
	err := c.EachValue(ctx, qs, func(v quad.Value) error {
		out = append(out, v)
		return nil
	})
	return out, err
}

// FirstValue is an analog of First, but it does lookup of a value in QuadStore.
func (c *Chain) FirstValue(ctx context.Context, qs refs.Namer) (quad.Value, error) {
	if qs != nil {
		c.qs = qs
	}
	if c.qs == nil {
		return nil, errNoQuadStore
	}
	v, err := c.First(ctx)
	if err != nil || v == nil {
		return nil, err
	}
	return c.qs.NameOf(ctx, v)
}

// SendValues is an analog of Send, but it will additionally call NameOf
// for each graph.Ref before sending it to a channel.
func (c *Chain) SendValues(ctx context.Context, qs refs.Namer, out chan<- quad.Value) error {
	if qs != nil {
		c.qs = qs
	}
	if c.qs == nil {
		return errNoQuadStore
	}
	c.start(ctx)
	defer c.end()
	done := ctx.Done()
	send := func(v refs.Ref) error {
		res, err := c.it.Result(ctx)
		if err != nil {
			return err
		}
		nv, err := c.qs.NameOf(ctx, res)
		if err != nil || nv == nil {
			return err
		}
		nvResult, err := c.qs.NameOf(ctx, res)
		if err != nil {
			return err
		}
		select {
		case <-done:
			return ctx.Err()
		case out <- nvResult:
		}
		return nil
	}

	process := func() error {
		res, err := c.it.Result(ctx)
		if err != nil {
			return err
		}
		return send(res)
	}

	for c.next(ctx) {
		if err := process(); err != nil {
			return err
		}
		for c.nextPath(ctx) {
			if err := process(); err != nil {
				return err
			}
		}
	}

	return c.it.Err()
}

// TagValues is an analog of TagEach, but it will additionally call NameOf
// for each graph.Ref before passing the map to a callback.
func (c *Chain) TagValues(ctx context.Context, qs refs.Namer, fnc func(map[string]quad.Value) error) error {
	if qs != nil {
		c.qs = qs
	}
	if c.qs == nil {
		return errNoQuadStore
	}
	return c.TagEach(ctx, func(m map[string]refs.Ref) error {
		vm := make(map[string]quad.Value, len(m))
		for k, v := range m {
			var err error
			vm[k], err = c.qs.NameOf(ctx, v) // TODO(dennwc): batch NameOf?
			if err != nil {
				return err
			}
		}
		fnc(vm)
		return nil
	})
}
