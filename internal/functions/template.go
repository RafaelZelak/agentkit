package functions

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ProcessTemplate processes a template string, replacing {{ function.name }} or {{ function.name(args) }} 
// with the result of executing the function
func ProcessTemplate(template string) (string, error) {
	result, _, err := ProcessTemplateWithTracking(template)
	return result, err
}

// ProcessTemplateWithTracking processes a template and returns the result along with a map of executed functions
// The map key is the original template (e.g., "{{ time.now }}") and the value is the result
func ProcessTemplateWithTracking(template string) (string, map[string]string, error) {
	// Pattern to match {{ ... }} including nested braces
	pattern := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	
	var result strings.Builder
	lastIndex := 0
	functionsUsed := make(map[string]string)
	
	matches := pattern.FindAllStringSubmatchIndex(template, -1)
	if len(matches) == 0 {
		return template, functionsUsed, nil
	}
	
	for _, match := range matches {
		// Add text before the match
		result.WriteString(template[lastIndex:match[0]])
		
		// Extract the full template including braces (e.g., "{{ time.now }}")
		fullTemplate := template[match[0]:match[1]]
		
		// Extract content inside {{ }}
		content := strings.TrimSpace(template[match[2]:match[3]])
		
		// Process the function call
		replacement, err := processFunctionCall(content)
		if err != nil {
			// On error, keep the original template or show error message
			errorMsg := fmt.Sprintf("{{ ERROR: %s }}", err.Error())
			result.WriteString(errorMsg)
			functionsUsed[fullTemplate] = errorMsg
		} else {
			result.WriteString(replacement)
			functionsUsed[fullTemplate] = replacement
		}
		
		lastIndex = match[1]
	}
	
	// Add remaining text
	result.WriteString(template[lastIndex:])
	
	return result.String(), functionsUsed, nil
}

// processFunctionCall parses and executes a function call like "time.now" or "time.greeting(\"user\")"
func processFunctionCall(content string) (string, error) {
	content = strings.TrimSpace(content)
	
	// Check if it has parentheses (function with arguments)
	if strings.Contains(content, "(") {
		return processFunctionWithArgs(content)
	}
	
	// Function without arguments
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

// processFunctionWithArgs parses a function call with arguments like "time.greeting(\"user\")"
func processFunctionWithArgs(content string) (string, error) {
	// Find the opening parenthesis
	openParen := strings.Index(content, "(")
	if openParen == -1 {
		return "", fmt.Errorf("invalid function call syntax: missing opening parenthesis")
	}
	
	functionName := strings.TrimSpace(content[:openParen])
	
	// Find the closing parenthesis
	closeParen := strings.LastIndex(content, ")")
	if closeParen == -1 || closeParen <= openParen {
		return "", fmt.Errorf("invalid function call syntax: missing closing parenthesis")
	}
	
	argsStr := strings.TrimSpace(content[openParen+1 : closeParen])
	
	// Parse arguments
	args, err := parseArguments(argsStr)
	if err != nil {
		return "", fmt.Errorf("error parsing arguments: %w", err)
	}
	
	// Get and call the function
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

// parseArguments parses a comma-separated list of arguments
// Supports strings (with quotes), integers, and floats
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
				// End of string - add the string value
				args = append(args, current.String())
				current.Reset()
				inString = false
				stringQuote = 0
			} else {
				current.WriteRune(char)
			}
		case ',':
			if !inString {
				// End of current argument
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
	
	// Handle last argument if no trailing comma
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

// parseValue parses a single value (string, int, or float)
func parseValue(s string) (interface{}, error) {
	s = strings.TrimSpace(s)
	
	// Try integer
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return int(i), nil
	}
	
	// Try float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}
	
	// Return as string
	return s, nil
}
