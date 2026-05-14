package quad

import (
	"errors"

	"github.com/aperturerobotics/fastjson"
)

func (q Quad) MarshalJSON() ([]byte, error) {
	var arena fastjson.Arena
	object := arena.NewObject()
	object.Set("subject", arena.NewString(ToString(q.Subject)))
	object.Set("predicate", arena.NewString(ToString(q.Predicate)))
	object.Set("object", arena.NewString(ToString(q.Object)))
	if q.Label != nil {
		label := ToString(q.Label)
		if label != "" {
			object.Set("label", arena.NewString(label))
		}
	}
	return object.MarshalTo(nil), nil
}

func (q *Quad) UnmarshalJSON(data []byte) error {
	var parser fastjson.Parser
	value, err := parser.ParseBytes(data)
	if err != nil {
		return err
	}
	object := value.GetObject()
	if object == nil {
		return errors.New("quad JSON must be an object")
	}
	subject, err := objectJSONString(object, "subject")
	if err != nil {
		return err
	}
	predicate, err := objectJSONString(object, "predicate")
	if err != nil {
		return err
	}
	objectValue, err := objectJSONString(object, "object")
	if err != nil {
		return err
	}
	label, err := objectJSONString(object, "label")
	if err != nil {
		return err
	}

	// TODO(dennwc): parse nquads? or use StringToValue hack?
	*q = MakeRaw(subject, predicate, objectValue, label)
	return nil
}

func objectJSONString(object *fastjson.Object, key string) (string, error) {
	value := object.Get(key)
	if value == nil || value.Type() == fastjson.TypeNull {
		return "", nil
	}
	if value.Type() != fastjson.TypeString {
		return "", errors.New("quad JSON field must be a string")
	}
	bytes, err := value.StringBytes()
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

type jsonMarshaler interface {
	MarshalJSON() ([]byte, error)
}

type jsonUnmarshaler interface {
	UnmarshalJSON([]byte) error
}

var (
	_ jsonMarshaler   = Quad{}
	_ jsonUnmarshaler = (*Quad)(nil)
)
