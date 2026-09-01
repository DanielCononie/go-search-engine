package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func LoadEnvironment() error {
	file, err := os.Open(".env")
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open .env: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		name, value, ok := strings.Cut(line, "=")
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if !ok || !validEnvironmentName(name) {
			return fmt.Errorf("invalid .env entry on line %d", lineNumber)
		}
		doubleQuoted := strings.HasPrefix(value, `"`) ||
			strings.HasSuffix(value, `"`)
		singleQuoted := strings.HasPrefix(value, "'") ||
			strings.HasSuffix(value, "'")
		if doubleQuoted {
			if len(value) < 2 ||
				value[0] != '"' || value[len(value)-1] != '"' {
				return fmt.Errorf("invalid .env value on line %d", lineNumber)
			}
			value, err = strconv.Unquote(value)
			if err != nil {
				return fmt.Errorf("invalid .env value on line %d: %w", lineNumber, err)
			}
		} else if singleQuoted {
			if len(value) < 2 ||
				value[0] != '\'' || value[len(value)-1] != '\'' {
				return fmt.Errorf("invalid .env value on line %d", lineNumber)
			}
			value = value[1 : len(value)-1]
		}
		if _, exists := os.LookupEnv(name); exists {
			continue
		}
		if err := os.Setenv(name, value); err != nil {
			return fmt.Errorf("set %s from .env: %w", name, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read .env: %w", err)
	}

	return nil
}

func validEnvironmentName(name string) bool {
	for index, character := range name {
		if character == '_' ||
			character >= 'A' && character <= 'Z' ||
			character >= 'a' && character <= 'z' ||
			index > 0 && character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return name != ""
}
