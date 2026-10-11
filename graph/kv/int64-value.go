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

import "github.com/aperturerobotics/cayley/graph/refs"

// Int64Value is a graph reference to a durable node ID.
type Int64Value uint64

// Key returns the node ID as a comparable reference key.
func (v Int64Value) Key() any { return v }

// _ is a type assertion.
var _ refs.Ref = Int64Value(0)
