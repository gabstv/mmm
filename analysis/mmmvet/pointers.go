package mmmvet

import (
	"go/types"
	"strings"
)

// containsPointers reports whether type t contains any GC-managed pointer.
// It breaks cycles using the seen map.
func containsPointers(t types.Type) bool {
	seen := make(map[types.Type]bool)
	return containsPointersRec(t, seen)
}

func containsPointersRec(t types.Type, seen map[types.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true

	switch u := t.(type) {
	case *types.Basic:
		return u.Info()&types.IsString != 0
	case *types.Pointer:
		if named, ok := u.Elem().(*types.Named); ok {
			if pkg := named.Obj().Pkg(); pkg != nil {
				if strings.HasPrefix(pkg.Path(), mmmPkgPath+"/") {
					return false
				}
			}
		}
		return true
	case *types.Slice:
		return true
	case *types.Map:
		return true
	case *types.Chan:
		return true
	case *types.Signature:
		return true
	case *types.Interface:
		return true
	case *types.Array:
		return containsPointersRec(u.Elem(), seen)
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if containsPointersRec(u.Field(i).Type(), seen) {
				return true
			}
		}
		return false
	case *types.Named:
		return containsPointersRec(u.Underlying(), seen)
	}
	return false
}

// pointerBearingFields returns dot-separated paths of fields that contain
// GC-managed pointers. t should be a struct type (or named struct).
func pointerBearingFields(t types.Type) []string {
	seen := make(map[types.Type]bool)
	var result []string
	collectPointerFields(t, "", seen, &result)
	return result
}

func collectPointerFields(t types.Type, prefix string, seen map[types.Type]bool, out *[]string) {
	if seen[t] {
		return
	}
	seen[t] = true

	switch u := t.(type) {
	case *types.Named:
		collectPointerFields(u.Underlying(), prefix, seen, out)
		return
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			f := u.Field(i)
			name := f.Name()
			if prefix != "" {
				name = prefix + "." + name
			}
			ft := f.Type()
			if containsPointers(ft) {
				// Check if the field itself is directly a pointer-bearing type or if
				// it's a struct that we should recurse into.
				underlying := ft
				if named, ok := ft.(*types.Named); ok {
					underlying = named.Underlying()
				}
				if s, ok := underlying.(*types.Struct); ok {
					// Recurse into nested structs
					subSeen := make(map[types.Type]bool)
					for k, v := range seen {
						subSeen[k] = v
					}
					collectPointerFields(s, name, subSeen, out)
				} else {
					*out = append(*out, name)
				}
			}
		}
	}
}
