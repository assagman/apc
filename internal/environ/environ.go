package environ

import (
	// "dac/pkg/logger"
	"fmt"
	"os"
	"strings"
)

func LoadEnv() error {
	// logger.Debug("Reading .env")
	dotenvBytes, readFileErr := os.ReadFile(".env")
	if readFileErr != nil {
		// logger.Warning("Unable to read .env %s.", readFileErr.Error())
		return readFileErr
	}
	dotenvStr := string(dotenvBytes)
	for line := range strings.Lines(dotenvStr) {
		line = strings.Trim(line, "\n")
		line = strings.Trim(line, " ")
		if line == "" {
			// logger.Debug("[LoadEnv]: line is empty")
			continue
		}
		if string(line[0]) == "#" {
			// logger.Debug("[LoadEnv]: line is comment")
			continue
		}

		firstEqualSign := strings.Index(line, "=")
		key := line[:firstEqualSign]
		val := line[firstEqualSign+1:]

		// handle double quotes
		if string(val[0]) == `"` && string(val[len(val)-1]) == `"` {
			val = val[1 : len(val)-1]
		}

		err := os.Setenv(key, val)
		if err != nil {
			// logger.Warning("Unable to set env %s : %e.", key, err) // TODO: must record missing values for displaying user that it's not possible to do related actions
		}
		// logger.Debug("Loaded %s", key)
	}
	return nil
}

func Get(envVarName string) (string, error) {
	value, exists := os.LookupEnv(envVarName)
	if !exists {
		return "", fmt.Errorf("%s is not set", envVarName)
	}
	return value, nil
}
