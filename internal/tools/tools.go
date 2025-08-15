package tools

func ConstructToolStruct(toolName string) Tool {
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

	return Tool{
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

func GetFsTools() ([]Tool, error) {
	var tools []Tool
	var funcMap = map[string]any{
		"ToolGetCurrentWorkingDirectory": ToolGetCurrentWorkingDirectory,
		"ToolGrepText":                   ToolGrepText,
		"ToolReadFile":                   ToolReadFile,
		"ToolTree":                       ToolTree,
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
