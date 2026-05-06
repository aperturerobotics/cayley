//go:build !tinygo

package shape

import "reflect"

func walkReflect(rv reflect.Value, fnc WalkFunc) {
	rt := rv.Type()
	switch rv.Kind() {
	case reflect.Slice:
		if rt.Elem().ConvertibleTo(rtShape) {
			// all element are shapes - call function on each of them
			for i := 0; i < rv.Len(); i++ {
				Walk(rv.Index(i).Interface().(Shape), fnc)
			}
		} else {
			// elements are not shapes, but might contain them
			for i := 0; i < rv.Len(); i++ {
				walkReflect(rv.Index(i), fnc)
			}
		}
	case reflect.Map:
		keys := rv.MapKeys()
		if rt.Elem().ConvertibleTo(rtShape) {
			// all element are shapes - call function on each of them
			for _, k := range keys {
				Walk(rv.MapIndex(k).Interface().(Shape), fnc)
			}
		} else {
			// elements are not shapes, but might contain them
			for _, k := range keys {
				walkReflect(rv.MapIndex(k), fnc)
			}
		}
	case reflect.Struct:
		// visit all fields
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			// if field is of shape type - call function on it
			// we skip anonymous fields because they were already visited as part of the parent
			if !f.Anonymous && f.Type.ConvertibleTo(rtShape) {
				Walk(rv.Field(i).Interface().(Shape), fnc)
				continue
			}
			// it might be a struct/map/slice field, so we need to go deeper
			walkReflect(rv.Field(i), fnc)
		}
	}
}
