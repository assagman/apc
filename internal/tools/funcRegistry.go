package tools

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

// ParamInfo holds information about a function parameter
type ParamInfo struct {
	TypeName    string
	Description string
}

// FunctionInfo holds the function, its parameter names, documentation, and param details
type FunctionInfo struct {
	fn          any
	paramNames  []string
	description string               // Main function description
	paramInfos  map[string]ParamInfo // Parameter-specific info
}

// FunctionRegistry holds a map of registered functions
type FunctionRegistry struct {
	functions map[string]FunctionInfo
}

// NewFunctionRegistry creates a new FunctionRegistry
func NewFunctionRegistry() *FunctionRegistry {
	fr := &FunctionRegistry{
		functions: make(map[string]FunctionInfo),
	}
	return fr
}

// RegisterFunction registers a function by extracting parameter names, types, and docs from source code
func (fr *FunctionRegistry) RegisterFunction(name string, fn any) error {
	// Validate that fn is a function
	fv := reflect.ValueOf(fn)
	if fv.Kind() != reflect.Func {
		return fmt.Errorf("provided value for %s is not a function", name)
	}

	// Get program counter (PC) for the function
	pc := fv.Pointer()
	rtFunc := runtime.FuncForPC(pc)
	if rtFunc == nil {
		return fmt.Errorf("could not find runtime function for %s", name)
	}

	// Get the file where the function is defined
	file, _ := rtFunc.FileLine(pc)

	// Determine the local function name from runtime
	fullName := rtFunc.Name()
	dotIdx := strings.LastIndex(fullName, ".")
	localName := fullName
	if dotIdx != -1 {
		localName = fullName[dotIdx+1:]
	}
	if localName == "" {
		return fmt.Errorf("could not determine local function name for %s", name)
	}

	// Parse the source file
	src, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read source file %s: %v", file, err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.ParseComments) // Enable comment parsing
	if err != nil {
		return fmt.Errorf("failed to parse source file %s: %v", file, err)
	}

	// Find the FuncDecl matching the name
	var funcDecl *ast.FuncDecl
	ast.Inspect(f, func(n ast.Node) bool {
		if fd, ok := n.(*ast.FuncDecl); ok {
			if fd.Name.Name == localName {
				funcDecl = fd
				return false
			}
		}
		return true
	})

	if funcDecl == nil {
		return fmt.Errorf("could not find function declaration for %s in %s", localName, file)
	}

	// Extract main description and parameter docs from comment
	var description string
	paramDocs := make(map[string]string)
	if funcDecl.Doc != nil {
		var inParamSection bool
		var currentLines []string
		for _, comment := range funcDecl.Doc.List {
			line := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(comment.Text, "//"), "/*"))
			line = strings.TrimSuffix(line, "*/")
			if line == "" {
				// Blank line separates main desc from param section
				if !inParamSection && len(currentLines) > 0 {
					description = strings.Join(currentLines, " ")
					currentLines = nil
					inParamSection = true
				}
				continue
			}
			if inParamSection {
				// Look for "paramName: description"
				if colonIdx := strings.Index(line, ":"); colonIdx != -1 {
					paramName := strings.TrimSpace(line[:colonIdx])
					paramDesc := strings.TrimSpace(line[colonIdx+1:])
					paramDocs[paramName] = paramDesc
				} else {
					// Append to previous param desc if no colon
					if len(paramDocs) > 0 {
						lastParam := "" // Get last key
						for k := range paramDocs {
							lastParam = k
						}
						paramDocs[lastParam] += " " + line
					}
				}
			} else {
				currentLines = append(currentLines, line)
			}
		}
		if !inParamSection && len(currentLines) > 0 {
			description = strings.Join(currentLines, " ")
		}
	}

	// Extract parameter names and types
	paramNames := make([]string, 0)
	paramInfos := make(map[string]ParamInfo)
	fnType := fv.Type()
	for i := 0; i < fnType.NumIn(); i++ {
		argType := fnType.In(i)
		// For param names, still extract from AST
		if i >= len(funcDecl.Type.Params.List) {
			return fmt.Errorf("parameter count mismatch in AST for %s", name)
		}
		// Assuming single name per param for simplicity; adjust if variadic or multiple
		paramList := funcDecl.Type.Params.List[i]
		if len(paramList.Names) != 1 {
			return fmt.Errorf("multiple names per param not supported for %s", name)
		}
		paramName := paramList.Names[0].Name
		if paramName == "_" {
			return fmt.Errorf("unnamed parameters (using _) are not supported for %s", name)
		}
		paramNames = append(paramNames, paramName)

		// Get type as string from reflection (could use AST for more details)
		typeName := mapGoTypeToJSONSchema(argType.String())

		// Get param doc if available
		paramDesc, ok := paramDocs[paramName]
		if !ok {
			paramDesc = "" // Default to empty
		}

		paramInfos[paramName] = ParamInfo{
			TypeName:    typeName,
			Description: paramDesc,
		}
	}

	// Validate number of parameters
	if len(paramNames) != fnType.NumIn() {
		return fmt.Errorf("mismatch in parameter count for %s: expected %d, found %d names", name, fnType.NumIn(), len(paramNames))
	}

	// Store in registry
	fr.functions[name] = FunctionInfo{
		fn:          fn,
		paramNames:  paramNames,
		description: description,
		paramInfos:  paramInfos,
	}
	return nil
}

// mapGoTypeToJSONSchema maps Go type strings to JSON schema types
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
	// Add more mappings as needed, e.g., slices -> array, structs -> object
	default:
		return "string" // Fallback
	}
}

// ExecFunc executes a registered function by name with provided named arguments
func (fr *FunctionRegistry) ExecFunc(funcName string, args map[string]any) (any, error) {
	// Check if function exists
	fnInfo, exists := fr.functions[funcName]
	if !exists {
		return nil, fmt.Errorf("function %s not found", funcName)
	}

	// Get function's reflection value and type
	fnValue := reflect.ValueOf(fnInfo.fn)
	fnType := fnValue.Type()

	// Validate function signature: should return (interface{}, error)
	if fnType.NumOut() != 2 {
		return nil, fmt.Errorf("function %s must return (interface{}, error)", funcName)
	}
	if !fnType.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return nil, fmt.Errorf("function %s second return value must be error", funcName)
	}

	// Prepare arguments using extracted parameter names
	inArgs, err := fr.prepareArguments(fnType, fnInfo.paramNames, args)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare arguments: %v", err)
	}

	// Call the function
	results := fnValue.Call(inArgs)

	// Extract results
	result := results[0].Interface()
	errInterface := results[1].Interface()

	if errInterface != nil {
		return nil, errInterface.(error)
	}

	return result, nil
}

// prepareArguments converts string map arguments to the types expected by the function
func (fr *FunctionRegistry) prepareArguments(fnType reflect.Type, paramNames []string, args map[string]any) ([]reflect.Value, error) {
	numIn := fnType.NumIn()
	inArgs := make([]reflect.Value, numIn)

	for i := range numIn {
		argType := fnType.In(i)
		paramName := paramNames[i]
		argValue, exists := args[paramName]
		if !exists {
			return nil, fmt.Errorf("missing argument %s", paramName)
		}

		// Convert string to appropriate type
		switch argType.Kind() {
		case reflect.String:
			inArgs[i] = reflect.ValueOf(argValue)
		case reflect.Int:
			argValueStr, ok := argValue.(string)
			if !ok {
				argValueStr = fmt.Sprintf("%v", argValue)
			}
			val, err := strconv.Atoi(argValueStr)
			if err != nil {
				return nil, fmt.Errorf("cannot convert %s to int: %v", argValueStr, err)
			}
			inArgs[i] = reflect.ValueOf(val)
		case reflect.Float64:
			argValueStr, ok := argValue.(string)
			if !ok {
				argValueStr = fmt.Sprintf("%v", argValue)
			}
			val, err := strconv.ParseFloat(argValueStr, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot convert %s to float64: %v", argValue, err)
			}
			inArgs[i] = reflect.ValueOf(val)
		case reflect.Bool:
			var val bool
			var err error
			switch v := argValue.(type) {
			case bool:
				val = v
			case string:
				val, err = strconv.ParseBool(v)
				if err != nil {
					return nil, fmt.Errorf("cannot convert %v to bool: %v", argValue, err)
				}
			default:
				return nil, fmt.Errorf("unsupported arg value type %T for bool param %s", argValue, paramName)
			}
			inArgs[i] = reflect.ValueOf(val)
		default:
			return nil, fmt.Errorf("unsupported argument type %s for %s", argType.Kind(), paramName)
		}
	}

	return inArgs, nil
}

var funcRegistry = NewFunctionRegistry()
