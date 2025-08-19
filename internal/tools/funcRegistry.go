// Package tools provides a FunctionRegistry that can register and execute
// ordinary functions *and* methods on structs.
package tools

import (
	"fmt"
	"go/ast"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ParamInfo holds JSON-schema style information for one parameter.
type ParamInfo struct {
	TypeName    string
	Description string
}

// FunctionInfo stores everything needed to execute a registered function/method.
type FunctionInfo struct {
	fn          any                  // the function/method itself
	receiver    reflect.Value        // receiver instance (for methods only)
	paramNames  []string             // names of *user* parameters (no receiver)
	description string               // doc comment above function/method
	paramInfos  map[string]ParamInfo // per-parameter metadata
}

// FunctionRegistry keeps a map of registered functions/methods.
type FunctionRegistry struct {
	functions map[string]FunctionInfo
}

// NewFunctionRegistry returns an empty registry.
func NewFunctionRegistry() *FunctionRegistry {
	return &FunctionRegistry{functions: make(map[string]FunctionInfo)}
}

// ---------- registration helpers ------------------------------------------

// RegisterFunction registers a standalone function.
func (fr *FunctionRegistry) RegisterFunction(name string, fn any) error {
	return fr.register(name, fn, nil, nil)
}

// RegisterMethods registers every exported method on 'instance'.
// Works for both value and pointer receivers.
func (fr *FunctionRegistry) RegisterMethods(instance any) error {
	val := reflect.ValueOf(instance)
	typ := val.Type()
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return fmt.Errorf("RegisterMethods: expected *struct or struct, got %s", typ.Kind())
	}

	// 1. Load package that contains the type.
	cfg := packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes,
	}
	pkgs, err := packages.Load(&cfg, typ.PkgPath())
	if err != nil {
		return fmt.Errorf("packages.Load: %w", err)
	}
	if len(pkgs) != 1 {
		return fmt.Errorf("expected 1 package for %s, got %d", typ.PkgPath(), len(pkgs))
	}
	pkg := pkgs[0]

	// 2. Build map: methodName -> *ast.FuncDecl
	scope := pkg.Types.Scope().Lookup(typ.Name())
	if scope == nil {
		return fmt.Errorf("type %s not found in package", typ.Name())
	}
	// named := scope.Type().(*types.Named)

	methodDecls := make(map[string]*ast.FuncDecl)
	for _, file := range pkg.Syntax {
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if ok && fn.Name.IsExported() && fn.Recv != nil {
				if recvType(fn.Recv) == "*"+typ.Name() || recvType(fn.Recv) == typ.Name() {
					methodDecls[fn.Name.Name] = fn
				}
			}
		}
	}

	// 3. Register each method found via reflection.
	for i := 0; i < reflect.TypeOf(instance).NumMethod(); i++ {
		method := reflect.TypeOf(instance).Method(i)
		decl, ok := methodDecls[method.Name]
		if !ok {
			// AST missing for this method (shouldn’t happen in normal builds)
			continue
		}
		if err := fr.register(method.Name, method.Func.Interface(), decl, &val); err != nil {
			return fmt.Errorf("method %s: %w", method.Name, err)
		}
	}
	return nil
}

// register is the internal helper for both standalone functions and methods.
func (fr *FunctionRegistry) register(name string, fn any, decl *ast.FuncDecl, receiver *reflect.Value) error {
	fv := reflect.ValueOf(fn)
	if fv.Kind() != reflect.Func {
		return fmt.Errorf("%s is not a function", name)
	}
	ft := fv.Type()

	// Collect parameter information.
	var paramNames []string
	paramInfos := make(map[string]ParamInfo)
	if decl != nil { // method
		for _, field := range decl.Type.Params.List {
			for _, ident := range field.Names {
				paramNames = append(paramNames, ident.Name)
				paramInfos[ident.Name] = ParamInfo{
					TypeName: mapGoTypeToJSONSchema(field.Type.(*ast.Ident).Name),
				}
			}
		}
	} else { // standalone function – simple reflection fallback
		for i := 0; i < ft.NumIn(); i++ {
			argName := fmt.Sprintf("arg%d", i)
			paramNames = append(paramNames, argName)
			paramInfos[argName] = ParamInfo{
				TypeName: mapGoTypeToJSONSchema(ft.In(i).String()),
			}
		}
	}

	desc := ""
	if decl != nil && decl.Doc != nil {
		desc = strings.TrimSpace(decl.Doc.Text())
	}

	receiverVal := reflect.Value{} // zero for standalone functions
	if receiver != nil {
		receiverVal = *receiver
	}

	fr.functions[name] = FunctionInfo{
		fn:          fn,
		receiver:    receiverVal,
		paramNames:  paramNames,
		description: desc,
		paramInfos:  paramInfos,
	}
	return nil
}

// ---------- execution -------------------------------------------------------

// ExecFunc executes a registered function/method with named parameters.
// Methods receive their receiver automatically.
func (fr *FunctionRegistry) ExecFunc(funcName string, args map[string]any) (any, error) {
	info, ok := fr.functions[funcName]
	if !ok {
		return nil, fmt.Errorf("function %s not found", funcName)
	}

	ft := reflect.TypeOf(info.fn)
	if ft.NumOut() != 2 {
		return nil, fmt.Errorf("function %s must return (result, error)", funcName)
	}
	if !ft.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return nil, fmt.Errorf("function %s second return must be error", funcName)
	}

	// Prepare only the user parameters (excluding receiver).
	prepared, err := prepareArguments(ft, info.paramNames, args)
	if err != nil {
		return nil, err
	}

	// Prepend receiver for methods.
	callArgs := make([]reflect.Value, 0, len(prepared)+1)
	if info.receiver.IsValid() {
		callArgs = append(callArgs, info.receiver)
	}
	callArgs = append(callArgs, prepared...)

	results := reflect.ValueOf(info.fn).Call(callArgs)
	if !results[1].IsNil() {
		return nil, results[1].Interface().(error)
	}
	return results[0].Interface(), nil
}

// prepareArguments converts the user-provided map into a reflect.Value slice.
func prepareArguments(fnType reflect.Type, paramNames []string, args map[string]any) ([]reflect.Value, error) {
	out := make([]reflect.Value, len(paramNames))
	for i, name := range paramNames {
		val, ok := args[name]
		if !ok {
			return nil, fmt.Errorf("missing argument %q", name)
		}

		var (
			argType reflect.Type
			err     error
		)
		// Receiver (index 0) is handled by ExecFunc, so we always skip it.
		argType = fnType.In(i + 1)

		converted, err := convertArg(val, argType)
		if err != nil {
			return nil, fmt.Errorf("argument %q: %w", name, err)
		}
		out[i] = converted
	}
	return out, nil
}

// convertArg converts an interface{} value to the required reflect.Value.
func convertArg(v any, target reflect.Type) (reflect.Value, error) {
	switch target.Kind() {
	case reflect.String:
		return reflect.ValueOf(fmt.Sprint(v)), nil
	case reflect.Int:
		s := fmt.Sprint(v)
		i, err := strconv.Atoi(s)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("cannot parse %q as int", s)
		}
		return reflect.ValueOf(i), nil
	case reflect.Float64:
		s := fmt.Sprint(v)
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("cannot parse %q as float", s)
		}
		return reflect.ValueOf(f), nil
	case reflect.Bool:
		switch t := v.(type) {
		case bool:
			return reflect.ValueOf(t), nil
		case string:
			b, err := strconv.ParseBool(t)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("cannot parse %q as bool", t)
			}
			return reflect.ValueOf(b), nil
		default:
			return reflect.Value{}, fmt.Errorf("unsupported bool value type %T", v)
		}
	default:
		return reflect.Value{}, fmt.Errorf("unsupported type %s", target)
	}
}

// ---------- misc -----------------------------------------------------------

// mapGoTypeToJSONSchema maps Go type strings to JSON schema types.
func mapGoTypeToJSONSchema(goType string) string {
	switch goType {
	case "string":
		return "string"
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return "integer"
	case "float32", "float64":
		return "number"
	case "bool":
		return "boolean"
	default:
		return "string"
	}
}

// recvType returns the string form of a method receiver.
func recvType(r *ast.FieldList) string {
	if r == nil || len(r.List) == 0 {
		return ""
	}
	switch t := r.List[0].Type.(type) {
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return "*" + id.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}

var funcRegistry = NewFunctionRegistry()
