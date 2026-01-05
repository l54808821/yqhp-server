package parser

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// variablePattern matches variable references like ${env:VAR}, ${secret:KEY}, ${var:name}
var variablePattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// DefaultVariableResolver provides default variable resolution from environment and secrets.
type DefaultVariableResolver struct {
	// Secrets holds secret values (in production, this would be backed by a secure store)
	Secrets map[string]string
	// Variables holds inline variable definitions
	Variables map[string]any
}

// NewDefaultVariableResolver creates a new DefaultVariableResolver.
func NewDefaultVariableResolver() *DefaultVariableResolver {
	return &DefaultVariableResolver{
		Secrets:   make(map[string]string),
		Variables: make(map[string]any),
	}
}

// WithSecrets sets the secrets map.
func (r *DefaultVariableResolver) WithSecrets(secrets map[string]string) *DefaultVariableResolver {
	r.Secrets = secrets
	return r
}

// WithVariables sets the variables map.
func (r *DefaultVariableResolver) WithVariables(variables map[string]any) *DefaultVariableResolver {
	r.Variables = variables
	return r
}

// Resolve resolves a variable reference.
// Supported formats:
//   - ${env:VAR_NAME} - resolves from environment variables
//   - ${secret:KEY} - resolves from secrets store
//   - ${var:name} - resolves from inline variables
//   - ${name} - resolves from inline variables (shorthand)
func (r *DefaultVariableResolver) Resolve(ref string) (any, error) {
	// Check if it's a full reference with prefix
	if strings.Contains(ref, ":") {
		parts := strings.SplitN(ref, ":", 2)
		if len(parts) != 2 {
			return nil, NewVariableResolutionError(ref, "invalid variable reference format", nil)
		}

		prefix := strings.ToLower(parts[0])
		key := parts[1]

		switch prefix {
		case "env":
			value, exists := os.LookupEnv(key)
			if !exists {
				return nil, NewVariableResolutionError(ref, fmt.Sprintf("environment variable '%s' not found", key), nil)
			}
			return value, nil

		case "secret":
			value, exists := r.Secrets[key]
			if !exists {
				return nil, NewVariableResolutionError(ref, fmt.Sprintf("secret '%s' not found", key), nil)
			}
			return value, nil

		case "var":
			value, exists := r.Variables[key]
			if !exists {
				return nil, NewVariableResolutionError(ref, fmt.Sprintf("variable '%s' not found", key), nil)
			}
			return value, nil

		default:
			return nil, NewVariableResolutionError(ref, fmt.Sprintf("unknown variable prefix '%s'", prefix), nil)
		}
	}

	// Shorthand: just the variable name
	value, exists := r.Variables[ref]
	if !exists {
		return nil, NewVariableResolutionError(ref, fmt.Sprintf("variable '%s' not found", ref), nil)
	}
	return value, nil
}

// ResolveString resolves all variable references in a string.
func (r *DefaultVariableResolver) ResolveString(s string) (string, error) {
	var lastErr error
	result := variablePattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract the reference (remove ${ and })
		ref := match[2 : len(match)-1]
		value, err := r.Resolve(ref)
		if err != nil {
			lastErr = err
			return match // Keep original on error
		}
		return fmt.Sprintf("%v", value)
	})

	if lastErr != nil {
		return "", lastErr
	}
	return result, nil
}

// HasVariableReferences checks if a string contains variable references.
func HasVariableReferences(s string) bool {
	return variablePattern.MatchString(s)
}

// ExtractVariableReferences extracts all variable references from a string.
func ExtractVariableReferences(s string) []string {
	matches := variablePattern.FindAllStringSubmatch(s, -1)
	refs := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			refs = append(refs, match[1])
		}
	}
	return refs
}
