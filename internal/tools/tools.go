package tools

import "fmt"

// GetUniqueId returns a unique ID string for use in various contexts.
//
// suffix: Optional suffix to append to the generated ID.
func GetUniqueId(suffix string) (any, error) {
	return "sda90123irohqwsjd98xdacde!#@$$@%^" + suffix, nil
}

func ConstructToolStruct(toolName string) *Tool {
	fnInfo := funcRegistry.functions[toolName]
	description := fnInfo.description
	if description == "" {
		description = "Get unique id string" // Fallback if no doc comment
	}

	// Populate parameters from extracted info
	properties := make(map[string]Property)
	required := make([]string, 0)
	for _, paramName := range fnInfo.paramNames {
		info := fnInfo.paramInfos[paramName]
		properties[paramName] = Property{
			Type:        info.TypeName,
			Description: info.Description,
		}
		required = append(required, paramName) // Assume all required
	}

	return &Tool{
		Type: "function",
		Function: FunctionDefinition{
			Name:        toolName,
			Description: description,
			Parameters: ToolFunctionParameters{
				Type:       "object",
				Properties: properties,
				Required:   required,
			},
		},
	}
}

func GetTool(toolName string) (*Tool, error) {
	if toolName == "GetUniqueId" {
		if err := funcRegistry.RegisterFunction(toolName, GetUniqueId); err != nil {
			return nil, err
		}
		return ConstructToolStruct(toolName), nil
	}
	if toolName == "GetCurrentWorkingDirectory" {
		if err := funcRegistry.RegisterFunction(toolName, ToolGetCurrentWorkingDirectory); err != nil {
			return nil, err
		}
		return ConstructToolStruct(toolName), nil
	}
	return nil, fmt.Errorf("Tool `%s` does not exist", toolName)
}

func GetFsTools() ([]*Tool, error) {
	var tools []*Tool
	var funcMap = map[string]any{
		"ToolGetCurrentWorkingDirectory": ToolGetCurrentWorkingDirectory,
		"ToolGrepText":                   ToolGrepText,
	}
	for funcName, fn := range funcMap {
		if err := funcRegistry.RegisterFunction(funcName, fn); err != nil {
			return nil, err
		}
		tools = append(tools, ConstructToolStruct(funcName))
	}
	return tools, nil
}

func ExecTool(funcName string, args map[string]any) (any, error) {
	return funcRegistry.ExecFunc(funcName, args)
}
