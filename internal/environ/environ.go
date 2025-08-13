package environ

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/assagman/apc/internal/logger"
)

func CheckEnvFile(envFile string) (bool, error) {
	if envFile == "" {
		logger.Warning("Empty env filename")
		return false, nil
	}
	if !path.IsAbs(envFile) {
		cwd, err := os.Getwd()
		if err != nil {
			logger.Error("Failed to get CWD")
		}
		envFile = path.Join(cwd, envFile)
	}
	stat, err := os.Stat(envFile)
	if os.IsNotExist(err) {
		logger.Warning("Given env file `%s` does not exist.")
		return false, nil
	}
	if stat.IsDir() {
		logger.Error("Given env file is a directory")
		return false, nil
	}

	return true, nil
}

func LoadEnv(envFile string) error {
	ok, err := CheckEnvFile(envFile)
	if err != nil {
		return err
	}
	if !ok {
		logger.Warning("Invalid env file `%s`. Falling back to `./.env", envFile)
		envFile = ".env"
	}
	// logger.Debug("Reading .env")
	dotenvBytes, readFileErr := os.ReadFile(envFile)
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
