package linkedql

import (
	"context"
	"fmt"

	"github.com/aperturerobotics/cayley/graph/refs"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/aperturerobotics/cayley/quad/jsonld"
	"github.com/aperturerobotics/cayley/query"
	"github.com/piprate/json-gold/ld"
)

var _ query.Iterator = (*TagsIterator)(nil)

// TagsIterator is a result iterator for records consisting of selected tags
// or all the tags in the query.
type TagsIterator struct {
	ValueIt   *ValueIterator
	Selected  []string
	ExcludeID bool
	err       error
}

// NewTagsIterator creates a new TagsIterator
func NewTagsIterator(valueIt *ValueIterator, selected []string, excludeID bool) TagsIterator {
	return TagsIterator{
		ValueIt:   valueIt,
		Selected:  selected,
		ExcludeID: excludeID,
		err:       nil,
	}
}

// Next implements query.Iterator.
func (it *TagsIterator) Next(ctx context.Context) bool {
	return it.ValueIt.Next(ctx)
}

func (it *TagsIterator) addQuadFromRef(ctx context.Context, dataset *ld.RDFDataset, subject ld.Node, tag string, ref refs.Ref) error {
	p := ld.NewIRI(tag)
	rname, err := it.ValueIt.Namer.NameOf(ctx, ref)
	if err != nil {
		return err
	}
	o, err := jsonld.ToNode(rname)
	if err != nil {
		return err
	}
	q := ld.NewQuad(subject, p, o, "")
	dataset.Graphs["@default"] = append(dataset.Graphs["@default"], q)
	return nil
}

func toSubject(ctx context.Context, namer refs.Namer, result refs.Ref) (ld.Node, error) {
	v, err := namer.NameOf(ctx, result)
	if err != nil {
		return nil, err
	}
	id, ok := v.(quad.Identifier)
	if !ok {
		return nil, fmt.Errorf("expected subject to be an entity identifier but instead received: %v", v)
	}
	return jsonld.ToNode(id)
}

func (it *TagsIterator) addResultsToDataset(ctx context.Context, dataset *ld.RDFDataset, result refs.Ref) error {
	s, err := toSubject(ctx, it.ValueIt.Namer, result)
	if err != nil {
		return err
	}

	refTags := make(map[string]refs.Ref)
	if err := it.ValueIt.scanner.TagResults(ctx, refTags); err != nil {
		return err
	}

	if len(it.Selected) == 0 {
		for tag, ref := range refTags {
			it.addQuadFromRef(ctx, dataset, s, tag, ref)
		}
	} else {
		for _, tag := range it.Selected {
			it.addQuadFromRef(ctx, dataset, s, tag, refTags[tag])
		}
	}

	return nil
}

// Result implements query.Iterator.
func (it *TagsIterator) Result(ctx context.Context) (any, error) {
	if err := it.err; err != nil {
		return nil, err
	}
	// FIXME(iddan): only convert when collation is JSON/JSON-LD, leave as Ref otherwise
	r, err := it.ValueIt.scanner.Result(ctx)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, nil
	}
	d := ld.NewRDFDataset()
	err = it.addResultsToDataset(ctx, d, r)
	if err != nil {
		it.err = err
		return nil, err
	}
	doc, err := singleDocumentFromRDF(d)
	if err != nil {
		it.err = err
		return nil, err
	}
	if !it.ExcludeID {
		m := doc.(map[string]any)
		delete(m, "@id")
		return m, nil
	}
	return doc, nil
}

// Err implements query.Iterator.
func (it *TagsIterator) Err() error {
	if it.err != nil {
		return it.err
	}
	return it.ValueIt.Err()
}

// Close implements query.Iterator.
func (it *TagsIterator) Close() error {
	return it.ValueIt.Close()
}
