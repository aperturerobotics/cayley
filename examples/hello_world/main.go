package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aperturerobotics/cayley"
	"github.com/aperturerobotics/cayley/quad"
)

func main() {
	// Create a brand new graph
	ctx := context.Background()
	store, err := cayley.NewMemoryGraph(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	store.AddQuad(ctx, quad.Make("phrase of the day", "is of course", "Hello World!", nil))

	// Now we create the path, to get to our data
	p := cayley.StartPath(store, quad.String("phrase of the day")).Out(quad.String("is of course"))

	// Now we iterate over results. Arguments:
	// 1. Optional context used for cancellation.
	// 2. Quad store, but we can omit it because we have already built path with it.
	err = p.Iterate(nil).EachValue(ctx, nil, func(value quad.Value) error {
		nativeValue := quad.NativeOf(value) // this converts RDF values to normal Go types
		fmt.Println(nativeValue)
		return nil
	})
	if err != nil {
		log.Fatalln(err)
	}
}
