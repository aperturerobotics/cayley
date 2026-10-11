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

package kv

import (
	"context"
	"errors"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/kv"
)

// Registration supplies the database constructors for a KV graph backend.
type Registration struct {
	// NewFunc opens a database for graph access.
	NewFunc NewFunc
	// InitFunc creates the persistent database before graph access.
	InitFunc InitFunc
	// IsPersistent reports whether the database survives its process.
	IsPersistent bool
}

// InitFunc creates a persistent database at a configured address.
type InitFunc func(string, graph.Options) (kv.KV, error)

// NewFunc opens a database at a configured address.
type NewFunc func(string, graph.Options) (kv.KV, error)

// Register exposes the backend through the graph store registry.
func Register(name string, r Registration) {
	graph.RegisterQuadStore(name, graph.QuadStoreRegistration{
		InitFunc: func(ctx context.Context, addr string, opt graph.Options) error {
			// Create and close a persistent database before it is opened as a graph.
			if !r.IsPersistent {
				return nil
			}
			db, err := r.InitFunc(addr, opt)
			if err != nil {
				return err
			}
			return db.Close()
		},
		NewFunc: func(ctx context.Context, addr string, opt graph.Options) (graph.QuadStore, error) {
			// Open the backing database for the graph handle.
			db, err := r.NewFunc(addr, opt)
			if err != nil {
				return nil, err
			}

			// A failed graph open must release the database it acquired.
			store, err := New(ctx, db, opt)
			if err != nil {
				return nil, errors.Join(err, db.Close())
			}
			return store, nil
		},
		IsPersistent: r.IsPersistent,
	})
}
