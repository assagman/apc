package exampleTools

import "strconv"

type ToolBox struct{}

func (t *ToolBox) ToolGetName() (string, error) {
	return "einstein", nil
}

func (t *ToolBox) ToolGetAge() (string, error) {
	return strconv.FormatInt(75, 10), nil
}
