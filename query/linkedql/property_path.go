package linkedql

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/aperturerobotics/cayley/quad/voc"
	"github.com/aperturerobotics/cayley/query/path"
)

// PropertyPathI is an interface to be used where a path of properties is expected.
type PropertyPathI interface {
	BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error)
}

// PropertyPath is a struct wrapping PropertyPathI
type PropertyPath struct {
	PropertyPathI
}

// NewPropertyPath constructs a new PropertyPath
func NewPropertyPath(p PropertyPathI) *PropertyPath {
	return &PropertyPath{PropertyPathI: p}
}

// Description implements Step.
func (*PropertyPath) Description() string {
	return "PropertyPath is a string, multiple strins or path describing a set of properties"
}

// UnmarshalJSON implements RawMessage
func (p *PropertyPath) UnmarshalJSON(data []byte) error {
	var errors []error

	var propertyIRIs PropertyIRIs
	err := json.Unmarshal(data, &propertyIRIs)
	if err == nil {
		p.PropertyPathI = propertyIRIs
		return nil
	}
	errors = append(errors, err)

	var propertyIRIStrings PropertyIRIStrings
	err = json.Unmarshal(data, &propertyIRIStrings)
	if err == nil {
		p.PropertyPathI = propertyIRIStrings
		return nil
	}
	errors = append(errors, err)

	var propertyIRI PropertyIRI
	err = json.Unmarshal(data, &propertyIRI)
	if err == nil {
		p.PropertyPathI = propertyIRI
		return nil
	}
	errors = append(errors, err)

	var propertyIRIString PropertyIRIString
	err = json.Unmarshal(data, &propertyIRIString)
	if err == nil {
		p.PropertyPathI = propertyIRIString
		return nil
	}
	errors = append(errors, err)

	step, err := Unmarshal(data)
	if err == nil {
		pathStep, ok := step.(PathStep)
		if ok {
			p.PropertyPathI = pathStep
			return nil
		}
		errors = append(errors, fmt.Errorf("Step of type %T is not a PathStep. A PropertyPath step must be a PathStep", step))
	}
	errors = append(errors, err)

	return formatMultiError(errors)
}

// PropertyIRIs is a slice of property IRIs.
type PropertyIRIs []PropertyIRI

// BuildPath implements PropertyPath.
func (p PropertyIRIs) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	var values []quad.Value
	for _, iri := range p {
		values = append(values, iri.full(ns))
	}
	return path.StartPath(qs, values...), nil
}

// PropertyIRIStrings is a slice of property IRI strings.
type PropertyIRIStrings []string

// PropertyIRIs casts PropertyIRIStrings into PropertyIRIs
func (p PropertyIRIStrings) PropertyIRIs() PropertyIRIs {
	var iris PropertyIRIs
	for _, iri := range p {
		iris = append(iris, PropertyIRI(iri))
	}
	return iris
}

// BuildPath implements PropertyPath.
func (p PropertyIRIStrings) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	return p.PropertyIRIs().BuildPath(ctx, qs, ns)
}

// PropertyIRI is an IRI of a Property
type PropertyIRI quad.IRI

func (p PropertyIRI) full(ns *voc.Namespaces) quad.IRI {
	return quad.IRI(p).FullWith(ns)
}

// BuildPath implements PropertyPath
func (p PropertyIRI) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	return path.StartPath(qs, p.full(ns)), nil
}

// PropertyIRIString is a string of IRI of a Property
type PropertyIRIString string

// BuildPath implements PropertyPath
func (p PropertyIRIString) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	iri := PropertyIRI(p)
	return iri.BuildPath(ctx, qs, ns)
}

// PropertyStep is a step that should resolve to a path of properties
type PropertyStep struct {
	PathStep
}

// BuildPath implements PropertyPath
func (p PropertyStep) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	return p.BuildPath(ctx, qs, ns)
}
