package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aperturerobotics/cayley"
	"github.com/aperturerobotics/cayley/quad"
)

func main() {
	// To see how most of this works, see hello_world -- this just add in a transaction
	ctx := context.Background()
	store, err := cayley.NewMemoryGraph(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	// Create a transaction of work to do
	// NOTE: the transaction is independent of the storage type, so comes from cayley rather than store
	t := cayley.NewTransaction()
	t.AddQuad(quad.Make("food", "is", "good", nil))
	t.AddQuad(quad.Make("phrase of the day", "is of course", "Hello World!", nil))
	t.AddQuad(quad.Make("cats", "are", "awesome", nil))
	t.AddQuad(quad.Make("cats", "are", "scary", nil))
	t.AddQuad(quad.Make("cats", "want to", "kill you", nil))

	// Apply the transaction
	err = store.ApplyTransaction(ctx, t)
	if err != nil {
		log.Fatalln(err)
	}

	p := cayley.StartPath(store, quad.String("cats")).Out(quad.String("are"))

	err = p.Iterate(nil).EachValue(ctx, nil, func(v quad.Value) error {
		fmt.Println("cats are", v.Native())
		return nil
	})
	if err != nil {
		log.Fatalln(err)
	}
}
