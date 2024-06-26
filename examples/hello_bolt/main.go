package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/aperturerobotics/cayley"
	"github.com/aperturerobotics/cayley/graph"
	_ "github.com/aperturerobotics/cayley/graph/kv/bolt"
	"github.com/aperturerobotics/cayley/quad"
)

func main() {
	// File for your new BoltDB. Use path to regular file and not temporary in the real world
	tmpdir, err := ioutil.TempDir("", "example")
	if err != nil {
		log.Fatal(err)
	}

	defer os.RemoveAll(tmpdir) // clean up

	// Initialize the database
	ctx := context.Background()
	err = graph.InitQuadStore(ctx, "bolt", tmpdir, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Open and use the database
	store, err := cayley.NewGraph(ctx, "bolt", tmpdir, nil)
	if err != nil {
		log.Fatalln(err)
	}

	store.AddQuad(ctx, quad.Make("phrase of the day", "is of course", "Hello BoltDB!", "demo graph"))

	// Now we create the path, to get to our data
	p := cayley.StartPath(store, quad.String("phrase of the day")).Out(quad.String("is of course"))

	// This is more advanced example of the query.
	// Simpler equivalent can be found in hello_world example.

	// Now we get an iterator for the path and optimize it.
	// The second return is if it was optimized, but we don't care for now.
	its, _, _ := p.BuildIterator(ctx).Optimize(ctx)
	it := its.Iterate(ctx)

	// remember to cleanup after yourself
	defer it.Close()

	// While we have items
	for it.Next(ctx) {
		token, err := it.Result(ctx) // get a ref to a node (backend-specific)
		if err != nil {
			log.Fatalln(err)
		}
		value, err := store.NameOf(ctx, token) // get the value in the node (RDF)
		if err != nil {
			log.Fatalln(err)
		}
		nativeValue := quad.NativeOf(value) // convert value to normal Go type

		fmt.Println(nativeValue) // print it!
	}
	if err := it.Err(); err != nil {
		log.Fatalln(err)
	}
}
