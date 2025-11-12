// Copyright 2017 The Cayley Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gizmo

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dop251/goja"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/iterator"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/aperturerobotics/cayley/quad/jsonld"
	"github.com/aperturerobotics/cayley/quad/voc"
	"github.com/aperturerobotics/cayley/query"
	"github.com/aperturerobotics/cayley/schema"
)

const Name = "gizmo"

func init() {
	query.RegisterLanguage(query.Language{
		Name: Name,
		Session: func(qs graph.QuadStore) query.Session {
			return NewSession(qs)
		},
	})
}

func NewSession(qs graph.QuadStore) *Session {
	s := &Session{
		ctx: context.Background(),
		sch: schema.NewConfig(),
		qs:  qs, limit: -1,
	}
	if err := s.buildEnv(); err != nil {
		panic(err)
	}
	return s
}

func lcFirst(str string) string {
	rune, size := utf8.DecodeRuneInString(str)
	return string(unicode.ToLower(rune)) + str[size:]
}

type fieldNameMapper struct{}

func (fieldNameMapper) FieldName(t reflect.Type, f reflect.StructField) string {
	return lcFirst(f.Name)
}

const (
	constructMethodPrefix        = "New"
	backwardsCompatibilityPrefix = "Capitalized"
)

func (fieldNameMapper) MethodName(t reflect.Type, m reflect.Method) string {
	if strings.HasPrefix(m.Name, backwardsCompatibilityPrefix) {
		return strings.TrimPrefix(m.Name, backwardsCompatibilityPrefix)
	}
	if strings.HasPrefix(m.Name, constructMethodPrefix) {
		return strings.TrimPrefix(m.Name, constructMethodPrefix)
	}
	return lcFirst(m.Name)
}

type Session struct {
	qs  graph.QuadStore
	vm  *goja.Runtime
	ns  voc.Namespaces
	sch *schema.Config
	col query.Collation

	last string
	p    *goja.Program

	out   chan *Result
	ctx   context.Context
	limit int
	count int
}

func (s *Session) context() context.Context {
	return s.ctx
}

func (s *Session) buildEnv() error {
	if s.vm != nil {
		return nil
	}
	s.vm = goja.New()
	s.vm.SetFieldNameMapper(fieldNameMapper{})
	s.vm.Set("graph", &graphObject{s: s})
	s.vm.Set("g", s.vm.Get("graph"))
	for name, val := range defaultEnv {
		fnc := val
		s.vm.Set(name, func(call goja.FunctionCall) goja.Value {
			return fnc(s.vm, call)
		})
	}
	return nil
}

func (s *Session) quadValueToNative(v quad.Value) interface{} {
	if v == nil {
		return nil
	}
	if s.col == query.JSONLD {
		return jsonld.FromValue(v)
	}
	out := v.Native()
	if nv, ok := out.(quad.Value); ok && v == nv {
		return quad.StringOf(v)
	}
	return out
}

func (s *Session) tagsToValueMap(m map[string]graph.Ref) (map[string]interface{}, error) {
	outputMap := make(map[string]interface{})
	for k, v := range m {
		nv, err := s.qs.NameOf(s.context(), v)
		if err != nil {
			return nil, err
		}
		if o := s.quadValueToNative(nv); o != nil {
			outputMap[k] = o
		}
	}
	if len(outputMap) == 0 {
		return nil, nil
	}
	return outputMap, nil
}

func (s *Session) runIteratorToArray(it iterator.Shape, limit int) ([]map[string]interface{}, error) {
	ctx := s.context()

	output := make([]map[string]interface{}, 0)
	err := iterator.Iterate(it).Limit(limit).TagEach(ctx, func(tags map[string]graph.Ref) error {
		tm, err := s.tagsToValueMap(tags)
		if err != nil {
			return err
		}
		if tm != nil {
			output = append(output, tm)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return output, nil
}

func (s *Session) runIteratorToArrayNoTags(it iterator.Shape, limit int) ([]interface{}, error) {
	ctx := s.context()

	output := make([]interface{}, 0)
	err := iterator.Iterate(it).Paths(false).Limit(limit).EachValue(ctx, s.qs, func(v quad.Value) error {
		if o := s.quadValueToNative(v); o != nil {
			output = append(output, o)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return output, nil
}

func (s *Session) runIteratorWithCallback(it iterator.Shape, callback goja.Value, this goja.FunctionCall, limit int) error {
	fnc, ok := goja.AssertFunction(callback)
	if !ok {
		return fmt.Errorf("expected js callback function")
	}
	ctx, cancel := context.WithCancel(s.context())
	defer cancel()
	return iterator.Iterate(it).Paths(true).Limit(limit).TagEach(ctx, func(tags map[string]graph.Ref) error {
		tm, err := s.tagsToValueMap(tags)
		if err != nil || tm == nil {
			return err
		}
		_, err = fnc(this.This, s.vm.ToValue(tm))
		if err != nil {
			cancel()
		}
		return err
	})
}

func (s *Session) send(ctx context.Context, r *Result) bool {
	if s.limit > 0 && s.count >= s.limit {
		return false
	}
	if s.out == nil {
		return false
	}
	if ctx == nil {
		ctx = s.ctx
	}
	select {
	case s.out <- r:
	case <-ctx.Done():
		return false
	}
	s.count++
	return s.limit <= 0 || s.count < s.limit
}

func (s *Session) runIterator(it iterator.Shape) error {
	ctx, cancel := context.WithCancel(s.context())
	defer cancel()
	stop := false
	err := iterator.Iterate(it).Paths(true).TagEach(ctx, func(tags map[string]graph.Ref) error {
		if !s.send(ctx, &Result{Tags: tags}) {
			cancel()
			stop = true
		}
		return nil
	})
	if stop {
		err = nil
	}
	return err
}

func (s *Session) countResults(it iterator.Shape) (int64, error) {
	ctx := s.context()
	return iterator.Iterate(it).Paths(true).Count(ctx)
}

type Result struct {
	Meta bool
	Val  interface{}
	Tags map[string]graph.Ref
}

func (r *Result) Result(ctx context.Context) (interface{}, error) {
	if r.Tags != nil {
		return r.Tags, nil
	}
	return r.Val, nil
}

func (s *Session) compile(qu string) error {
	if s.last == qu && s.last != "" {
		return nil
	}
	p, err := goja.Compile("", qu, false)
	if err != nil {
		return err
	}
	s.last, s.p = qu, p
	return nil
}

func (s *Session) run() (goja.Value, error) {
	v, err := s.vm.RunProgram(s.p)
	if e, ok := err.(*goja.Exception); ok && e.Value() != nil {
		if er, ok := e.Value().Export().(error); ok {
			err = er
		}
	}
	return v, err
}

func (s *Session) Execute(ctx context.Context, qu string, opt query.Options) (query.Iterator, error) {
	switch opt.Collation {
	case query.Raw, query.JSON, query.JSONLD, query.REPL:
	default:
		return nil, &query.ErrUnsupportedCollation{Collation: opt.Collation}
	}
	if err := s.compile(qu); err != nil {
		return nil, err
	}
	s.limit = opt.Limit
	s.count = 0
	ctx, cancel := context.WithCancel(ctx)
	s.ctx = ctx
	s.col = opt.Collation
	return &results{
		col: opt.Collation,
		s:   s,
		ctx: ctx, cancel: cancel,
	}, nil
}

type results struct {
	s      *Session
	col    query.Collation
	ctx    context.Context
	cancel func()

	running bool
	errc    chan error

	err error
	cur *Result
}

func (it *results) stop(err error) {
	it.cancel()
	if !it.running {
		return
	}
	it.s.vm.Interrupt(err)
	it.running = false
}

func (it *results) Next(ctx context.Context) bool {
	if it.errc == nil {
		it.s.out = make(chan *Result)
		it.errc = make(chan error, 1)
		it.running = true
		go func() {
			defer close(it.errc)
			v, err := it.s.run()
			if err != nil {
				it.errc <- err
				return
			}
			if !goja.IsNull(v) && !goja.IsUndefined(v) {
				it.s.send(it.ctx, &Result{Meta: true, Val: v.Export()})
			}
		}()
	}
	select {
	case r := <-it.s.out:
		it.cur = r
		return true
	case err := <-it.errc:
		if err != nil {
			it.err = err
		}
		return false
	case <-ctx.Done():
		it.err = ctx.Err()
		it.stop(it.err)
		return false
	}
}

func (it *results) Result(ctx context.Context) (interface{}, error) {
	if err := it.Err(); err != nil {
		return nil, err
	}
	if it.cur == nil {
		return nil, nil
	}
	switch it.col {
	case query.Raw:
		return it.cur, nil
	case query.JSON, query.JSONLD:
		return it.jsonResult(ctx)
	case query.REPL:
		return it.replResult(ctx)
	}
	return nil, nil
}

func (it *results) jsonResult(ctx context.Context) (interface{}, error) {
	data := it.cur
	if data.Meta {
		return nil, nil
	}
	if data.Val != nil {
		return data.Val, nil
	}
	obj := make(map[string]interface{})
	tags := data.Tags
	var tagKeys []string
	for k := range tags {
		tagKeys = append(tagKeys, k)
	}
	sort.Strings(tagKeys)
	for _, k := range tagKeys {
		name, err := it.s.qs.NameOf(ctx, tags[k])
		if err != nil {
			it.err = err
			return nil, err
		}
		if name != nil {
			obj[k] = it.s.quadValueToNative(name)
		} else {
			delete(obj, k)
		}
	}
	return obj, nil
}

func (it *results) replResult(ctx context.Context) (interface{}, error) {
	data := it.cur
	if data.Meta {
		if data.Val != nil {
			s := data.Val
			switch s.(type) {
			case *pathObject, *graphObject:
				s = "[internal Iterator]"
			}
			return fmt.Sprintln("=>", s), nil
		}
		return fmt.Sprintln("=>", nil), nil
	}
	var out string
	out = fmt.Sprintln("****")
	if data.Val == nil {
		tags := data.Tags
		tagKeys := make([]string, len(tags))
		i := 0
		for k := range tags {
			tagKeys[i] = k
			i++
		}
		sort.Strings(tagKeys)
		for _, k := range tagKeys {
			if k == "$_" {
				continue
			}
			knv, err := it.s.qs.NameOf(ctx, tags[k])
			if err != nil {
				return nil, err
			}
			out += fmt.Sprintf("%s : %s\n", k, quadValueToString(knv))
		}
	} else {
		switch export := data.Val.(type) {
		case map[string]string:
			for k, v := range export {
				out += fmt.Sprintf("%s : %s\n", k, v)
			}
		case map[string]interface{}:
			for k, v := range export {
				out += fmt.Sprintf("%s : %v\n", k, v)
			}
		default:
			out += fmt.Sprintf("%s\n", data.Val)
		}
	}
	return out, nil
}

func (it *results) Err() error {
	return it.err
}

func (it *results) Close() error {
	it.stop(errors.New("iterator closed"))
	return nil
}
