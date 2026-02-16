package schema

import (
	"encoding/json"
	"reflect"
	"strconv"

	"github.com/aperturerobotics/cayley/query/linkedql"
	// Steps are imported here so they be registered and documented in the schema
	"github.com/aperturerobotics/cayley/quad"
	"github.com/aperturerobotics/cayley/quad/voc/owl"
	"github.com/aperturerobotics/cayley/quad/voc/rdf"
	"github.com/aperturerobotics/cayley/quad/voc/rdfs"
	"github.com/aperturerobotics/cayley/quad/voc/xsd"
	_ "github.com/aperturerobotics/cayley/query/linkedql/steps"
)

// rdfgGraph is the W3C type for named graphs
const (
	rdfgNamespace = "http://www.w3.org/2004/03/trix/rdfg-1/"
	rdfgPrefix    = "rdfg:"
	rdfgGraph     = rdfgPrefix + "Graph"
)

var (
	pathStep         = reflect.TypeFor[linkedql.PathStep]()
	iteratorStep     = reflect.TypeFor[linkedql.IteratorStep]()
	entityIdentifier = reflect.TypeFor[linkedql.EntityIdentifier]()
	value            = reflect.TypeFor[quad.Value]()
	propertyPath     = reflect.TypeFor[*linkedql.PropertyPath]()
	stringMap        = reflect.TypeFor[map[string]string]()
	graphPattern     = reflect.TypeFor[linkedql.GraphPattern]()
)

func typeToRange(t reflect.Type) string {
	if t == stringMap {
		return "rdf:JSON"
	}
	if t == graphPattern {
		return rdfgGraph
	}
	if t.Kind() == reflect.Slice {
		return typeToRange(t.Elem())
	}
	if t.Kind() == reflect.String {
		return xsd.String
	}
	if t.Kind() == reflect.Bool {
		return xsd.Boolean
	}
	if kind := t.Kind(); kind == reflect.Int64 || kind == reflect.Int {
		return xsd.Int
	}
	if t.Implements(value) {
		return rdfs.Resource
	}
	if t == entityIdentifier {
		return owl.Thing
	}
	if t == pathStep {
		return linkedql.Prefix + "PathStep"
	}
	if t == propertyPath {
		return linkedql.Prefix + "PropertyPath"
	}
	panic("Unexpected type " + t.String())
}

// identified is used for referencing a type
type identified struct {
	ID string `json:"@id"`
}

// newIdentified creates new identified struct
func newIdentified(id string) identified {
	return identified{ID: id}
}

// cardinalityRestriction is used to indicate a how many values can a property get
type cardinalityRestriction struct {
	ID          string     `json:"@id"`
	Type        string     `json:"@type"`
	Cardinality int        `json:"owl:cardinality"`
	Property    identified `json:"owl:onProperty"`
}

func newBlankNodeID() string {
	return quad.RandomBlankNode().String()
}

// newSingleCardinalityRestriction creates a cardinality of 1 restriction for given property
func newSingleCardinalityRestriction(prop string) cardinalityRestriction {
	return cardinalityRestriction{
		ID:          newBlankNodeID(),
		Type:        owl.Restriction,
		Cardinality: 1,
		Property:    identified{ID: prop},
	}
}

type owlPropertyRestriction struct {
	ID       string     `json:"@id"`
	Type     string     `json:"@type"`
	Property identified `json:"owl:onProperty"`
}

func newOWLPropertyRestriction(prop string) owlPropertyRestriction {
	return owlPropertyRestriction{
		ID:       newBlankNodeID(),
		Type:     owl.Restriction,
		Property: identified{ID: prop},
	}
}

// minCardinalityRestriction is used to indicate a how many values can a property get at the very least
type minCardinalityRestriction struct {
	owlPropertyRestriction
	MinCardinality int `json:"owl:minCardinality"`
}

// maxCardinalityRestriction is used to indicate a how many values can a property get at most
type maxCardinalityRestriction struct {
	owlPropertyRestriction
	MaxCardinality int `json:"owl:maxCardinality"`
}

func newMinCardinalityRestriction(prop string, minCardinality int) minCardinalityRestriction {
	return minCardinalityRestriction{
		owlPropertyRestriction: newOWLPropertyRestriction(prop),
		MinCardinality:         minCardinality,
	}
}

func newSingleMaxCardinalityRestriction(prop string) maxCardinalityRestriction {
	return maxCardinalityRestriction{
		owlPropertyRestriction: newOWLPropertyRestriction(prop),
		MaxCardinality:         1,
	}
}

// getOWLPropertyType for given kind of value type returns property OWL type
func getOWLPropertyType(kind reflect.Kind) string {
	if kind == reflect.String || kind == reflect.Bool || kind == reflect.Int64 || kind == reflect.Int {
		return owl.DatatypeProperty
	}
	return owl.ObjectProperty
}

// property is used to declare a property
type property struct {
	ID     string `json:"@id"`
	Type   string `json:"@type"`
	Domain any    `json:"rdfs:domain"`
	Range  any    `json:"rdfs:range"`
}

// class is used to declare a class
type class struct {
	ID           string `json:"@id"`
	Type         string `json:"@type"`
	Comment      string `json:"rdfs:comment"`
	SuperClasses []any  `json:"rdfs:subClassOf"`
}

// newClass creates a new class struct
func newClass(id string, superClasses []any, comment string) class {
	return class{
		ID:           id,
		Type:         rdfs.Class,
		SuperClasses: superClasses,
		Comment:      comment,
	}
}

// getStepTypeClasses for given step type returns the matching class identifiers
func getStepTypeClasses(t reflect.Type) []string {
	var typeClasses []string
	if t.Implements(pathStep) {
		typeClasses = append(typeClasses, linkedql.Prefix+"PathStep")
	}
	if t.Implements(iteratorStep) {
		typeClasses = append(typeClasses, linkedql.Prefix+"IteratorStep")
	}
	return typeClasses
}

type list struct {
	Members []any `json:"@list"`
}

func newList(members []any) list {
	return list{
		Members: members,
	}
}

type unionOf struct {
	ID   string `json:"@id"`
	Type string `json:"@type"`
	List list   `json:"owl:unionOf"`
}

func newUnionOf(classes []string) unionOf {
	var members []any
	for _, class := range classes {
		members = append(members, newIdentified(class))
	}
	return unionOf{
		ID:   newBlankNodeID(),
		Type: owl.Class,
		List: newList(members),
	}
}

func newGenerator() *generator {
	return &generator{
		propToTypes:   make(map[string]map[string]struct{}),
		propToDomains: make(map[string]map[string]struct{}),
		propToRanges:  make(map[string]map[string]struct{}),
	}
}

type generator struct {
	out           []any
	propToTypes   map[string]map[string]struct{}
	propToDomains map[string]map[string]struct{}
	propToRanges  map[string]map[string]struct{}
}

// returns super types
func (g *generator) addTypeFields(name string, t reflect.Type, indirect bool) []any {
	var super []any
	for j := 0; j < t.NumField(); j++ {
		f := t.Field(j)
		if f.Anonymous {
			if f.Type.Kind() != reflect.Struct || !indirect {
				continue
			}
			super = append(super, g.addTypeFields(name, f.Type, false)...)
			continue
		}
		prop := linkedql.Prefix + f.Tag.Get("json")
		var hasMinCardinality bool
		v, ok := f.Tag.Lookup("minCardinality")
		if ok {
			minCardinality, err := strconv.Atoi(v)
			if err != nil {
				panic(err)
			}
			hasMinCardinality = true
			super = append(super, newMinCardinalityRestriction(prop, minCardinality))
		}
		if f.Type.Kind() != reflect.Slice {
			if hasMinCardinality {
				super = append(super, newSingleMaxCardinalityRestriction(prop))
			} else {
				super = append(super, newSingleCardinalityRestriction(prop))
			}
		}
		typ := getOWLPropertyType(f.Type.Kind())

		if g.propToTypes[prop] == nil {
			g.propToTypes[prop] = make(map[string]struct{})
		}
		g.propToTypes[prop][typ] = struct{}{}

		if g.propToDomains[prop] == nil {
			g.propToDomains[prop] = make(map[string]struct{})
		}
		g.propToDomains[prop][name] = struct{}{}

		if g.propToRanges[prop] == nil {
			g.propToRanges[prop] = make(map[string]struct{})
		}
		g.propToRanges[prop][typeToRange(f.Type)] = struct{}{}
	}
	return super
}

func (g *generator) AddType(name string, t reflect.Type) {
	step, ok := reflect.New(t).Interface().(linkedql.Step)
	if !ok {
		return
	}
	var super []any
	stepTypeClasses := getStepTypeClasses(reflect.PointerTo(t))
	for _, typeClass := range stepTypeClasses {
		super = append(super, newIdentified(typeClass))
	}
	super = append(super, g.addTypeFields(name, t, true)...)
	g.out = append(g.out, newClass(name, super, step.Description()))
}

func (g *generator) Generate() []byte {
	for prop, types := range g.propToTypes {
		if len(types) != 1 {
			panic("Properties must be either object properties or datatype properties. " + prop + " has both.")
		}
		var typ string
		for t := range types {
			typ = t
			break
		}
		var domains []string
		for d := range g.propToDomains[prop] {
			domains = append(domains, d)
		}
		var ranges []string
		for r := range g.propToRanges[prop] {
			ranges = append(ranges, r)
		}
		var dom any
		if len(domains) == 1 {
			dom = identified{domains[0]}
		} else {
			dom = newUnionOf(domains)
		}
		var rng any
		if len(ranges) == 1 {
			rng = newIdentified(ranges[0])
		} else {
			rng = newUnionOf(ranges)
		}
		g.out = append(g.out, property{
			ID:     prop,
			Type:   typ,
			Domain: dom,
			Range:  rng,
		})
	}
	graph := []any{
		map[string]string{
			"@id":   linkedql.Prefix + "Step",
			"@type": owl.Class,
		},
		map[string]any{
			"@id":           linkedql.Prefix + "PathStep",
			"@type":         owl.Class,
			rdfs.SubClassOf: identified{ID: linkedql.Prefix + "Step"},
		},
		map[string]any{
			"@id":           linkedql.Prefix + "IteratorStep",
			"@type":         owl.Class,
			rdfs.SubClassOf: identified{ID: linkedql.Prefix + "Step"},
		},
	}
	graph = append(graph, g.out...)
	data, err := json.Marshal(map[string]any{
		"@context": map[string]any{
			"rdf":      rdf.NS,
			"rdfs":     rdfs.NS,
			"owl":      owl.NS,
			"xsd":      xsd.NS,
			"linkedql": linkedql.Namespace,
			"rdfg":     rdfgNamespace,
		},
		"@graph": graph,
	})
	if err != nil {
		panic(err)
	}
	return data
}

// Generate a schema in JSON-LD format that contains all registered LinkedQL types and properties.
func Generate() []byte {
	g := newGenerator()
	for _, name := range linkedql.RegisteredTypes() {
		t, ok := linkedql.TypeByName(name)
		if !ok {
			panic("type is registered, but the lookup fails")
		}
		g.AddType(name, t)
	}
	return g.Generate()
}
