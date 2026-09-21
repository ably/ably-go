package ablyutil

import "reflect"

// Merge iterates over fields of struct pointed by v and when it's non-zero,
// copies its value to corresponding filed in orig.
//
// merge assumes both orig and v are pointers to a struct value of the
// same type.
//
// When defaults is true, merge uses v as the source of default values for each
// field; the default is copied when orig's field is a zero-value.
func Merge(orig, v interface{}, defaults bool) {
	vv := reflect.ValueOf(v).Elem()
	if !vv.IsValid() {
		return
	}
	vorig := reflect.ValueOf(orig).Elem()
	for i := 0; i < vorig.NumField(); i++ {
		field := vv.Field(i)
		if defaults {
			field = vorig.Field(i)
		}
		var empty bool
		switch field.Type().Kind() {
		case reflect.Struct:
			empty = true // TODO: merge structs recursively
		case reflect.Chan, reflect.Func, reflect.Slice, reflect.Map:
			empty = field.IsNil()
		default:
			empty = field.Interface() == reflect.Zero(field.Type()).Interface()
		}
		if !empty {
			vorig.Field(i).Set(vv.Field(i))
		}
	}
}
