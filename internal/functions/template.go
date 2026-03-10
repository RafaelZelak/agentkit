package functions

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func ProcessTemplate(template string) (string, error) {
	result, _, err := ProcessTemplateWithTracking(template)
	return result, err
}

func ProcessTemplateWithTracking(template string) (string, map[string]string, error) {
	pattern := regexp.MustCompile(`\{\{([^}]+)\}\}`)

	var result strings.Builder
	lastIndex := 0
	functionsUsed := make(map[string]string)

	matches := pattern.FindAllStringSubmatchIndex(template, -1)
	if len(matches) == 0 {
		return template, functionsUsed, nil
	}

	for _, match := range matches {
		result.WriteString(template[lastIndex:match[0]])

		fullTemplate := template[match[0]:match[1]]

		content := strings.TrimSpace(template[match[2]:match[3]])

		replacement, err := processFunctionCall(content)
		if err != nil {
			errorMsg := fmt.Sprintf("{{ ERROR: %s }}", err.Error())
			result.WriteString(errorMsg)
			functionsUsed[fullTemplate] = errorMsg
		} else {
			result.WriteString(replacement)
			functionsUsed[fullTemplate] = replacement
		}

		lastIndex = match[1]
	}

	result.WriteString(template[lastIndex:])

	return result.String(), functionsUsed, nil
}

func processFunctionCall(content string) (string, error) {
	content = strings.TrimSpace(content)

	if strings.Contains(content, "(") {
		return processFunctionWithArgs(content)
	}

	fn := Get(content)
	if fn == nil {
		return "", fmt.Errorf("function '%s' not found", content)
	}

	result, err := fn()
	if err != nil {
		return "", fmt.Errorf("error executing '%s': %w", content, err)
	}

	return result, nil
}

func processFunctionWithArgs(content string) (string, error) {
	openParen := strings.Index(content, "(")
	if openParen == -1 {
		return "", fmt.Errorf("invalid function call syntax: missing opening parenthesis")
	}

	functionName := strings.TrimSpace(content[:openParen])

	closeParen := strings.LastIndex(content, ")")
	if closeParen == -1 || closeParen <= openParen {
		return "", fmt.Errorf("invalid function call syntax: missing closing parenthesis")
	}

	argsStr := strings.TrimSpace(content[openParen+1 : closeParen])

	args, err := parseArguments(argsStr)
	if err != nil {
		return "", fmt.Errorf("error parsing arguments: %w", err)
	}

	fn := Get(functionName)
	if fn == nil {
		return "", fmt.Errorf("function '%s' not found", functionName)
	}

	result, err := fn(args...)
	if err != nil {
		return "", fmt.Errorf("error executing '%s': %w", functionName, err)
	}

	return result, nil
}

func parseArguments(argsStr string) ([]interface{}, error) {
	if argsStr == "" {
		return []interface{}{}, nil
	}

	argsStr = strings.TrimSpace(argsStr)
	var args []interface{}
	var current strings.Builder
	inString := false
	stringQuote := rune(0)
	escapeNext := false

	for _, char := range argsStr {
		if escapeNext {
			if inString {
				switch char {
				case 'n':
					current.WriteRune('\n')
				case 't':
					current.WriteRune('\t')
				case 'r':
					current.WriteRune('\r')
				case '\\':
					current.WriteRune('\\')
				case '"', '\'':
					current.WriteRune(char)
				default:
					current.WriteRune('\\')
					current.WriteRune(char)
				}
			} else {
				current.WriteRune(char)
			}
			escapeNext = false
			continue
		}

		switch char {
		case '\\':
			if inString {
				escapeNext = true
			} else {
				current.WriteRune(char)
			}
		case '"', '\'':
			if !inString {
				inString = true
				stringQuote = char
			} else if char == stringQuote {
				args = append(args, current.String())
				current.Reset()
				inString = false
				stringQuote = 0
			} else {
				current.WriteRune(char)
			}
		case ',':
			if !inString {
				argStr := strings.TrimSpace(current.String())
				if argStr != "" {
					parsed, err := parseValue(argStr)
					if err != nil {
						return nil, fmt.Errorf("error parsing argument '%s': %w", argStr, err)
					}
					args = append(args, parsed)
				}
				current.Reset()
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}

	if !inString {
		argStr := strings.TrimSpace(current.String())
		if argStr != "" {
			parsed, err := parseValue(argStr)
			if err != nil {
				return nil, fmt.Errorf("error parsing argument '%s': %w", argStr, err)
			}
			args = append(args, parsed)
		}
	} else {
		return nil, fmt.Errorf("unclosed string literal")
	}

	return args, nil
}

func parseValue(s string) (interface{}, error) {
	s = strings.TrimSpace(s)

	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return int(i), nil
	}

	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}

	return s, nil
}
