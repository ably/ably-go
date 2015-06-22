package testutil

import (
	"reflect"

	"github.com/ably/ably-go/ably"
)

// merge iterates over fields of struct pointed by extra and when it's non-zero,
// copies its value to corresponding filed in orig.
//
// merge assumes both orig and extra are pointers to a struct value of the
// same type.
//
// When defaults is true, merge uses v as the source of default values for each
// field; the default is copied when orig's field is a zero-value.
//
// NOTE: the implementation of merge is copied from ably.go due to avoid
// recursive deps problem.
func merge(orig, v interface{}, defaults bool) {
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

func mergeOpts(opts, extra *ably.ClientOptions) *ably.ClientOptions {
	if extra == nil {
		return opts
	}
	merge(opts, extra, false)
	merge(&opts.AuthOptions, &extra.AuthOptions, false)
	return opts
}

func MergeOptions(opts ...*ably.ClientOptions) *ably.ClientOptions {
	switch len(opts) {
	case 0:
		return nil
	case 1:
		return opts[0]
	}
	mergedOpts := opts[0]
	for _, opt := range opts[1:] {
		mergedOpts = mergeOpts(mergedOpts, opt)
	}
	return mergedOpts
}
