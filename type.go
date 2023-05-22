package goloader

import (
	"cmd/objfile/goobj"
	"cmd/objfile/obj"
	"cmd/objfile/objabi"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"unsafe"
)

type tflag uint8

// See runtime/type.go _typePair
type _typePair struct {
	t1 *_type
	t2 *_type
}

// See reflect/value.go emptyInterface
type emptyInterface struct {
	_type *_type
	data  unsafe.Pointer
}

type nonEmptyInterface struct {
	// see ../runtime/iface.go:/Itab
	itab *itab
	word unsafe.Pointer
}

func efaceOf(ep *interface{}) *emptyInterface {
	return (*emptyInterface)(unsafe.Pointer(ep))
}

// See reflect/value.go sliceHeader
type sliceHeader struct {
	Data uintptr
	Len  int
	Cap  int
}

// Method on non-interface type
type method struct {
	name nameOff // name of method
	mtyp typeOff // method type (without receiver)
	ifn  textOff // fn used in interface call (one-word receiver)
	tfn  textOff // fn used for normal method call
}

type imethod struct {
	name nameOff
	ityp typeOff
}

type interfacetype struct {
	typ     _type
	pkgpath name
	mhdr    []imethod
}

type name struct {
	bytes *byte
}

//go:linkname _uncommon runtime.(*_type).uncommon
func _uncommon(t *_type) *uncommonType

//go:linkname typesEqual runtime.typesEqual
func typesEqual(t, v *_type, seen map[_typePair]struct{}) bool

//go:linkname _nameOff runtime.(*_type).nameOff
func _nameOff(t *_type, off nameOff) name

//go:linkname _typeOff runtime.(*_type).typeOff
func _typeOff(t *_type, off typeOff) *_type

//go:linkname _name runtime.name.name
func _name(n name) string

//go:linkname _pkgPath runtime.name.pkgPath
func _pkgPath(n name) string

//go:linkname _isExported runtime.name.isExported
func _isExported(n name) bool

//go:linkname _methods reflect.(*uncommonType).methods
func _methods(t *uncommonType) []method

//go:linkname _Kind reflect.(*rtype).Kind
func _Kind(t *_type) reflect.Kind

//go:linkname _Elem reflect.(*rtype).Elem
func _Elem(t *_type) *_type

func (t *_type) uncommon() *uncommonType    { return _uncommon(t) }
func (t *_type) nameOff(off nameOff) name   { return _nameOff(t, off) }
func (t *_type) typeOff(off typeOff) *_type { return _typeOff(t, off) }
func (n name) name() string                 { return _name(n) }
func (n name) pkgPath() string              { return _pkgPath(n) }
func (n name) isExported() bool             { return _isExported(n) }
func (t *uncommonType) methods() []method   { return _methods(t) }
func (t *_type) Kind() reflect.Kind         { return _Kind(t) }
func (t *_type) Elem() *_type               { return _Elem(t) }

// This replaces local package names with import paths, including where the package name doesn't match the last part of the import path e.g.
//
//	import "github.com/org/somepackage/v4" + somepackage.SomeStruct
//	 =>  github.com/org/somepackage/v4.SomeStruct
func resolveFullyQualifiedSymbolName(t *_type) string {
	typ := AsRType(t)
	pkgPath := objabi.PathToPrefix(typ.PkgPath())
	name := typ.Name()
	if pkgPath != "" && name != "" {
		return pkgPath + "." + name
	}
	switch t.Kind() {
	case reflect.Ptr:
		return "*" + resolveFullyQualifiedSymbolName(fromRType(typ.Elem()))
	case reflect.Struct:
		if typ.NumField() == 0 {
			return typ.String()
		}
		fields := make([]string, typ.NumField())
		for i := 0; i < typ.NumField(); i++ {
			fieldName := typ.Field(i).Name + " "
			if typ.Field(i).Anonymous {
				fieldName = ""
			}
			fieldPkgPath := ""
			if typ.Field(i).PkgPath != "" && typ.Field(i).Type.PkgPath() == "" {
				fieldPkgPath = objabi.PathToPrefix(typ.Field(i).PkgPath) + "."
			}
			fieldStructTag := ""
			if typ.Field(i).Tag != "" {
				fieldStructTag = fmt.Sprintf(" %q", string(typ.Field(i).Tag))
			}
			fields[i] = fmt.Sprintf("%s%s%s%s", fieldPkgPath, fieldName, resolveFullyQualifiedSymbolName(fromRType(typ.Field(i).Type)), fieldStructTag)
		}
		return fmt.Sprintf("struct { %s }", strings.Join(fields, "; "))
	case reflect.Map:
		return fmt.Sprintf("map[%s]%s", resolveFullyQualifiedSymbolName(fromRType(typ.Key())), resolveFullyQualifiedSymbolName(fromRType(typ.Elem())))
	case reflect.Chan:
		switch reflect.ChanDir(typ.ChanDir()) {
		case reflect.BothDir:
			return fmt.Sprintf("chan %s", resolveFullyQualifiedSymbolName(fromRType(typ.Elem())))
		case reflect.RecvDir:
			return fmt.Sprintf("<-chan %s", resolveFullyQualifiedSymbolName(fromRType(typ.Elem())))
		case reflect.SendDir:
			return fmt.Sprintf("chan<- %s", resolveFullyQualifiedSymbolName(fromRType(typ.Elem())))
		}
	case reflect.Slice:
		return fmt.Sprintf("[]%s", resolveFullyQualifiedSymbolName(fromRType(typ.Elem())))
	case reflect.Array:
		return fmt.Sprintf("[%d]%s", typ.Len(), resolveFullyQualifiedSymbolName(fromRType(typ.Elem())))
	case reflect.Func:
		ins := make([]string, typ.NumIn())
		outs := make([]string, typ.NumOut())
		for i := 0; i < typ.NumIn(); i++ {
			ins[i] = resolveFullyQualifiedSymbolName(fromRType(typ.In(i)))
			if i == typ.NumIn()-1 && typ.IsVariadic() {
				ins[i] = "..." + resolveFullyQualifiedSymbolName(fromRType(typ.In(i).Elem()))
			}
		}
		for i := 0; i < typ.NumOut(); i++ {
			outs[i] = resolveFullyQualifiedSymbolName(fromRType(typ.Out(i)))
		}
		funcName := "func(" + strings.Join(ins, ", ") + ")"
		if len(outs) > 0 {
			funcName += " "
		}
		if len(outs) > 1 {
			funcName += "("
		}
		funcName += strings.Join(outs, ", ")
		if len(outs) > 1 {
			funcName += ")"
		}
		return funcName
	case reflect.Interface:
		if goobj.BuiltinIdx(TypePrefix+typ.String(), int(obj.ABI0)) != -1 {
			// must be a builtin,
			return typ.String()
		}
		if typ.NumMethod() == 0 {
			return typ.String()
		}
		methods := make([]string, typ.NumMethod())
		ifaceT := (*interfacetype)(unsafe.Pointer(t))

		for i := 0; i < typ.NumMethod(); i++ {
			methodType := _typeOff(t, ifaceT.mhdr[i].ityp)
			methodName := _nameOff(t, ifaceT.mhdr[i].name).name()
			methods[i] = fmt.Sprintf("%s(%s", methodName, strings.TrimPrefix(resolveFullyQualifiedSymbolName(methodType), "func("))
		}
		reflect.TypeOf(0)
		return fmt.Sprintf("interface { %s }", strings.Join(methods, "; "))
	default:
		if goobj.BuiltinIdx(TypePrefix+typ.String(), int(obj.ABI0)) != -1 {
			// must be a builtin,
			return typ.String()
		}
		switch typ.String() {
		case "int", "uint", "struct {}", "interface {}":
			return typ.String()
		}
		panic("unexpected builtin type: " + typ.String())
	}
	return ""
}

func symbolIsVariant(name string) (string, bool) {
	const dot = "·"
	const noAlgPrefix = TypePrefix + "noalg."
	if strings.HasPrefix(name, TypePrefix+"struct {") || strings.HasPrefix(name, TypePrefix+"*struct {") {
		// Anonymous structs might embed variant types, so these will need parsing first
		ptr := false
		if strings.HasPrefix(name, TypePrefix+"*struct {") {
			ptr = true
		}
		fieldsStr := strings.TrimPrefix(name, TypePrefix+"struct { ")
		fieldsStr = strings.TrimPrefix(name, TypePrefix+"*struct { ")
		fieldsStr = strings.TrimSuffix(fieldsStr, " }")
		fields := strings.Split(fieldsStr, "; ")
		isVariant := false
		for j, field := range fields {
			var typeName string
			var typeNameIndex int
			fieldTypeTag := strings.SplitN(field, " ", 3)
			// could be anonymous, or tagless, or both - we want to operate on the type
			switch len(fieldTypeTag) {
			case 1:
				// Anonymous, tagless - just a type
				typeName = fieldTypeTag[0]
			case 2:
				// could be a name + type, or type + tag
				if strings.HasPrefix(fieldTypeTag[1], "\"") || strings.HasPrefix(fieldTypeTag[1], "`") {
					// type + tag
					typeName = fieldTypeTag[0]
				} else {
					// name + type
					typeName = fieldTypeTag[1]
					typeNameIndex = 1
				}
			case 3:
				// Name + type + tag
				typeName = fieldTypeTag[1]
				typeNameIndex = 1
			}
			i := len(typeName)
			for i > 0 && typeName[i-1] >= '0' && typeName[i-1] <= '9' {
				i--
			}
			if i >= len(dot) && typeName[i-len(dot):i] == dot {
				isVariant = true
				fieldTypeTag[typeNameIndex] = typeName[:i-len(dot)]
				fields[j] = strings.Join(fieldTypeTag, " ")
			}
		}
		if isVariant {
			if ptr {
				return TypePrefix + "*struct { " + strings.Join(fields, "; ") + " }", true
			}
			return TypePrefix + "struct { " + strings.Join(fields, "; ") + " }", true

		}
		return "", false
	} else {
		// need to double check for function scoped types which get a ·N suffix added, and also type.noalg.* variants
		i := len(name)
		for i > 0 && name[i-1] >= '0' && name[i-1] <= '9' {
			i--
		}
		if i >= len(dot) && name[i-len(dot):i] == dot {
			return name[:i-len(dot)], true
		} else if strings.HasPrefix(name, noAlgPrefix) {
			return TypePrefix + strings.TrimPrefix(name, noAlgPrefix), true
		}
		return "", false
	}
}

func funcPkgPath(funcName string) string {
	funcName = strings.TrimPrefix(funcName, TypeDoubleDotPrefix+"eq.")
	// Anonymous struct methods can't have a package
	if strings.HasPrefix(funcName, "go"+ObjSymbolSeparator+"struct {") || strings.HasPrefix(funcName, "go"+ObjSymbolSeparator+"(*struct {") {
		return ""
	}
	lastSlash := strings.LastIndexByte(funcName, '/')
	if lastSlash == -1 {
		lastSlash = 0
	}
	// Generic dictionaries
	firstDict := strings.Index(funcName, "..dict")
	if firstDict > 0 {
		return funcName[:firstDict]
	} else {
		// Methods on structs embedding structs from other packages look funny, e.g.:
		// regexp.(*onePassInst).regexp/syntax.op
		firstBracket := strings.LastIndex(funcName, ".(")
		if firstBracket > 0 && lastSlash > firstBracket {
			lastSlash = firstBracket
		}
	}
	dot := lastSlash
	for ; dot < len(funcName) && funcName[dot] != '.' && funcName[dot] != '(' && funcName[dot] != '['; dot++ {
	}
	pkgPath := funcName[:dot]
	return strings.TrimPrefix(strings.TrimPrefix(pkgPath, TypePrefix+".eq."), "[...]")
}

func (t *_type) PkgPath() string {
	ut := t.uncommon()
	if ut == nil {
		return EmptyString
	}
	return t.nameOff(ut.pkgpath).name()
}

func RegTypes(symPtr map[string]uintptr, interfaces ...interface{}) {
	for _, inter := range interfaces {
		v := reflect.ValueOf(inter)
		regType(symPtr, v)
		if v.Kind() == reflect.Ptr {
			regType(symPtr, v.Elem())
		}
	}
}

func regType(symPtr map[string]uintptr, v reflect.Value) {
	inter := v.Interface()
	if v.Kind() == reflect.Func && getFunctionPtr(inter) != 0 {
		symPtr[runtime.FuncForPC(v.Pointer()).Name()] = getFunctionPtr(inter)
	} else {
		header := (*emptyInterface)(unsafe.Pointer(&inter))
		t := header._type
		registerType(t, symPtr, map[string]struct{}{})
	}
}

func buildModuleTypeHash(module *moduledata, typeHash map[uint32][]*_type) {
collect:
	for _, tl := range module.typelinks {
		var t *_type
		t = (*_type)(adduintptr(module.types, int(tl)))

		// Add to typeHash if not seen before, and indirect pointer types to add both
		if t.Kind() == reflect.Ptr {
			element := *(**_type)(add(unsafe.Pointer(t), unsafe.Sizeof(_type{})))
			var elementElem *_type
			if element != nil {
				switch element.Kind() {
				case reflect.Slice, reflect.Chan, reflect.Array, reflect.Ptr:
					elementElem = *(**_type)(add(unsafe.Pointer(element), unsafe.Sizeof(_type{})))
				}
			}

			if element != nil && element.Kind() != reflect.Invalid {
				tlist := typeHash[element.hash]
				shouldAdd := true
				for _, tcur := range tlist {
					if tcur == element {
						shouldAdd = false
						break
					}
				}
				if shouldAdd {
					typeHash[element.hash] = append(tlist, element)
				}
				if elementElem != nil && elementElem.Kind() != reflect.Invalid {
					tlist := typeHash[elementElem.hash]
					shouldAdd := true
					for _, tcur := range tlist {
						if tcur == elementElem {
							shouldAdd = false
							break
						}
					}
					if shouldAdd {
						typeHash[elementElem.hash] = append(tlist, elementElem)
					}
				}
			}
		}
		tlist := typeHash[t.hash]
		for _, tcur := range tlist {
			if tcur == t {
				continue collect
			}
		}
		typeHash[t.hash] = append(tlist, t)
	}
}
