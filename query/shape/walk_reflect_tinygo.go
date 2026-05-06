//go:build tinygo

package shape

import "reflect"

func walkReflect(rv reflect.Value, fnc WalkFunc) {
	rt := rv.Type()
	switch rv.Kind() {
	case reflect.Slice:
		if rt.Elem().ConvertibleTo(rtShape) {
			for i := 0; i < rv.Len(); i++ {
				Walk(rv.Index(i).Interface().(Shape), fnc)
			}
			return
		}
		for i := 0; i < rv.Len(); i++ {
			walkReflect(rv.Index(i), fnc)
		}
	case reflect.Struct:
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			if !f.Anonymous && f.Type.ConvertibleTo(rtShape) {
				Walk(rv.Field(i).Interface().(Shape), fnc)
				continue
			}
			walkReflect(rv.Field(i), fnc)
		}
	}
}
