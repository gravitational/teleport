// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// packageAliases maps each Go package path the generated output file may reference
// to the alias used in its import block. If the printer encounters a type
// from a package not in this map it panics. Only GA Kubernetes API groups the chart
// is likely to use are listed.
var packageAliases = map[string]string{
	"k8s.io/api/admissionregistration/v1":                      "admissionregistrationv1",
	"k8s.io/api/apps/v1":                                       "appsv1",
	"k8s.io/api/autoscaling/v1":                                "autoscalingv1",
	"k8s.io/api/autoscaling/v2":                                "autoscalingv2",
	"k8s.io/api/batch/v1":                                      "batchv1",
	"k8s.io/api/certificates/v1":                               "certificatesv1",
	"k8s.io/api/coordination/v1":                               "coordinationv1",
	"k8s.io/api/core/v1":                                       "corev1",
	"k8s.io/api/discovery/v1":                                  "discoveryv1",
	"k8s.io/api/events/v1":                                     "eventsv1",
	"k8s.io/api/networking/v1":                                 "networkingv1",
	"k8s.io/api/node/v1":                                       "nodev1",
	"k8s.io/api/policy/v1":                                     "policyv1",
	"k8s.io/api/rbac/v1":                                       "rbacv1",
	"k8s.io/api/scheduling/v1":                                 "schedulingv1",
	"k8s.io/api/storage/v1":                                    "storagev1",
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1": "apiextensionsv1",
	"k8s.io/apimachinery/pkg/apis/meta/v1":                     "metav1",
	"k8s.io/apimachinery/pkg/api/resource":                     "resource",
	"k8s.io/apimachinery/pkg/util/intstr":                      "intstr",
}

// printer constructs Go source code from a runtime.Object.
type printer struct {
	sb           strings.Builder
	indent       int
	usedAliases  map[string]bool
	rootTypeName string
}

func newPrinter(usedAliases map[string]bool) *printer {
	return &printer{usedAliases: usedAliases}
}

func (p *printer) String() string {
	return p.sb.String()
}

// writeRootPointer emits `&pkg.Type{...}` for a pointer to a struct
// and records rootTypeName for the constructor's return type.
func (p *printer) writeRootPointer(obj runtime.Object) {
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		panic(fmt.Sprintf("writeRootPointer: need a non-nil pointer, got %T", obj))
	}

	elem := v.Elem()
	if elem.Kind() != reflect.Struct {
		panic(fmt.Sprintf("writeRootPointer: need a pointer to a struct, got %s", elem.Kind()))
	}

	p.rootTypeName = p.goType(elem.Type())
	p.sb.WriteByte('&')
	p.writeStruct(elem)
}

// writeValue emits any value. The enclosing type name is always written.
func (p *printer) writeValue(v reflect.Value) {
	switch fullTypeName(v.Type()) {
	case "k8s.io/apimachinery/pkg/api/resource.Quantity":
		// Quantity.String has a pointer receiver, so the type assertion
		// result must land in a local variable to be addressable.
		q := v.Interface().(resource.Quantity)
		p.usedAliases["resource"] = true
		fmt.Fprintf(&p.sb, "resource.MustParse(%q)", q.String())
		return
	case "k8s.io/apimachinery/pkg/util/intstr.IntOrString":
		p.usedAliases["intstr"] = true
		ios := v.Interface().(intstr.IntOrString)
		if ios.Type == intstr.Int {
			fmt.Fprintf(&p.sb, "intstr.FromInt32(%d)", ios.IntVal)
		} else {
			fmt.Fprintf(&p.sb, "intstr.FromString(%q)", ios.StrVal)
		}
		return
	}

	switch v.Kind() {
	case reflect.Bool:
		p.writeScalar(v, strconv.FormatBool(v.Bool()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		p.writeScalar(v, strconv.FormatInt(v.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		p.writeScalar(v, strconv.FormatUint(v.Uint(), 10))
	case reflect.String:
		p.writeString(v)
	case reflect.Pointer:
		if v.IsNil() {
			p.sb.WriteString("nil")
			return
		}
		p.sb.WriteString("ptr(")
		p.writeValueInPtr(v.Elem())
		p.sb.WriteByte(')')
	case reflect.Slice:
		p.writeSlice(v)
	case reflect.Map:
		p.writeMap(v)
	case reflect.Struct:
		p.writeStruct(v)
	case reflect.Interface:
		if v.IsNil() {
			p.sb.WriteString("nil")
			return
		}
		p.writeValue(v.Elem())
	default:
		panic(fmt.Sprintf("unhandled reflect.Kind %s (type %s)", v.Kind(), v.Type()))
	}
}

// writeScalar emits a scalar literal, casting to a named type if any.
func (p *printer) writeScalar(v reflect.Value, lit string) {
	if isNamedType(v.Type()) {
		fmt.Fprintf(&p.sb, "%s(%s)", p.goType(v.Type()), lit)
		return
	}
	p.sb.WriteString(lit)
}

// writeValueInPtr emits the argument via the ptr(...) helper. Typed integers
// need an explicit cast because ptr's generic T inference defaults int literals to
// int, and the target field is *int32, *int64, etc.
func (p *printer) writeValueInPtr(v reflect.Value) {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fmt.Fprintf(&p.sb, "%s(%d)", p.goType(v.Type()), v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fmt.Fprintf(&p.sb, "%s(%d)", p.goType(v.Type()), v.Uint())
	default:
		p.writeValue(v)
	}
}

// writeString emits a string literal. Named-type strings are wrapped in a cast.
func (p *printer) writeString(v reflect.Value) {
	if isNamedType(v.Type()) {
		fmt.Fprintf(&p.sb, "%s(%q)", p.goType(v.Type()), v.String())
		return
	}
	fmt.Fprintf(&p.sb, "%q", v.String())
}

// writeStruct emits a struct literal, omitting zero-valued and unexported
// fields.
func (p *printer) writeStruct(v reflect.Value) {
	t := v.Type()
	p.sb.WriteString(p.goType(t))
	open := false
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() || v.Field(i).IsZero() {
			continue
		}
		if !open {
			p.sb.WriteString("{\n")
			p.indent++
			open = true
		}
		p.writeIndent()
		p.sb.WriteString(f.Name)
		p.sb.WriteString(": ")
		p.writeValue(v.Field(i))
		p.sb.WriteString(",\n")
	}
	if !open {
		p.sb.WriteString("{}")
		return
	}
	p.indent--
	p.writeIndent()
	p.sb.WriteByte('}')
}

// writeSlice emits a slice literal. Unnamed []byte is rendered as []byte("...").
func (p *printer) writeSlice(v reflect.Value) {
	t := v.Type()
	if t.Elem().Kind() == reflect.Uint8 && !isNamedType(t.Elem()) {
		fmt.Fprintf(&p.sb, "[]byte(%q)", string(v.Bytes()))
		return
	}
	p.sb.WriteString(p.goType(t))
	if v.Len() == 0 {
		p.sb.WriteString("{}")
		return
	}
	p.sb.WriteString("{\n")
	p.indent++
	for i := 0; i < v.Len(); i++ {
		p.writeIndent()
		p.writeValue(v.Index(i))
		p.sb.WriteString(",\n")
	}
	p.indent--
	p.writeIndent()
	p.sb.WriteByte('}')
}

// writeMap emits a map literal with deterministic key order.
func (p *printer) writeMap(v reflect.Value) {
	t := v.Type()
	p.sb.WriteString(p.goType(t))
	if v.Len() == 0 {
		p.sb.WriteString("{}")
		return
	}
	keys := v.MapKeys()
	slices.SortFunc(keys, func(a, b reflect.Value) int { return strings.Compare(a.String(), b.String()) })
	p.sb.WriteString("{\n")
	p.indent++
	for _, k := range keys {
		p.writeIndent()
		p.writeValue(k)
		p.sb.WriteString(": ")
		p.writeValue(v.MapIndex(k))
		p.sb.WriteString(",\n")
	}
	p.indent--
	p.writeIndent()
	p.sb.WriteByte('}')
}

func (p *printer) writeIndent() {
	p.sb.WriteString(strings.Repeat("\t", p.indent))
}

// goType returns the Go source identifier for t, looking up the package
// alias in packageAliases. Panics for packages not in the map.
func (p *printer) goType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Pointer:
		return "*" + p.goType(t.Elem())
	case reflect.Slice:
		return "[]" + p.goType(t.Elem())
	case reflect.Map:
		return "map[" + p.goType(t.Key()) + "]" + p.goType(t.Elem())
	}

	if t.PkgPath() == "" {
		return t.Name()
	}

	alias, ok := packageAliases[t.PkgPath()]
	if !ok {
		panic(fmt.Sprintf("no alias for package %q (type %s); add an entry to packageAliases", t.PkgPath(), t))
	}

	p.usedAliases[alias] = true
	return alias + "." + t.Name()
}

// isNamedType reports whether t is a defined type rather than an unnamed builtin.
func isNamedType(t reflect.Type) bool {
	return t.PkgPath() != "" && t.Name() != ""
}

// fullTypeName returns "pkg/path.TypeName" for special-case dispatch.
func fullTypeName(t reflect.Type) string {
	if t.PkgPath() == "" {
		return ""
	}
	return t.PkgPath() + "." + t.Name()
}
