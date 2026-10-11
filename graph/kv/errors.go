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

import "github.com/pkg/errors"

var (
	// ErrNoBucket indicates that the graph has no stored metadata value.
	ErrNoBucket = errors.New("kv: no bucket")
	// ErrEmptyPath indicates that a persistent database path was omitted.
	ErrEmptyPath = errors.New("kv: path to the database must be specified")
)
