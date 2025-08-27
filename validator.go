package godepfind

import (
	"bufio"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// GoFileValidator provides methods to validate Go files before processing
type GoFileValidator struct{}

// NewGoFileValidator creates a new validator instance
func NewGoFileValidator() *GoFileValidator {
	return &GoFileValidator{}
}

// ValidateInputForProcessing validates handler and file before processing
// This function provides centralized validation that can be reused across multiple API endpoints.
//
// It performs the following validations:
// 1. Handler validation (nil check and main file path validation)
// 2. Go file validation (syntax, completeness, and write-in-progress detection)
//
// Returns:
//   - shouldProcess: true if processing should continue, false if file should be skipped
//   - error: validation error that should be returned to caller, or nil if validation passed
//
// Usage patterns:
//   - shouldProcess=true, error=nil: Continue with normal processing
//   - shouldProcess=false, error=nil: Skip processing (file is being written, empty, etc.)
//   - shouldProcess=false, error!=nil: Return error to caller (invalid handler, etc.)
func (g *GoDepFind) ValidateInputForProcessing(mainFilePath, fileName, filePath string) (bool, error) {
	// Validate handler's main file path
	if mainFilePath == "" {
		return false, fmt.Errorf("handler main file path cannot be empty")
	}

	// Validate Go file before processing
	if filePath != "" && filepath.Ext(fileName) == ".go" {
		validator := NewGoFileValidator()

		// Resolve relative paths from the root directory
		resolvedPath := filePath
		if !filepath.IsAbs(filePath) {
			// Check if filePath already starts with rootDir
			if strings.HasPrefix(filePath, g.rootDir+"/") || filePath == g.rootDir {
				// Path already includes rootDir, use as is
				resolvedPath = filePath
			} else {
				// Path doesn't include rootDir, join them
				resolvedPath = filepath.Join(g.rootDir, filePath)
			}
		}

		// Check if file is valid
		isValid, err := validator.IsValidGoFile(resolvedPath)
		if err != nil {
			return false, fmt.Errorf("file validation failed: %w", err)
		}

		// If file is not valid, check if it's being written
		if !isValid {
			isBeingWritten, err := validator.IsFileBeingWritten(resolvedPath)
			if err != nil {
				return false, fmt.Errorf("file write detection failed: %w", err)
			}

			// If file is being written, skip processing to avoid breaking the parser
			if isBeingWritten {
				return false, nil // File is being written, skip for now
			}

			// If file is empty or invalid and not being written, skip processing
			return false, nil
		}
	}
	// If filePath is empty, we can't validate file existence, but that's OK
	// The caller will use fileName-based lookup instead

	return true, nil
}

// IsValidGoFile checks if a Go file is valid and safe to process
func (v *GoFileValidator) IsValidGoFile(filePath string) (bool, error) {
	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		return false, err
	}

	// Check if file is empty
	if info.Size() == 0 {
		return false, nil // Empty files are not valid Go files
	}

	// Check file extension
	if filepath.Ext(filePath) != ".go" {
		return false, nil
	}

	// Check if file has valid Go syntax
	return v.hasValidGoSyntax(filePath)
}

// hasValidGoSyntax checks if the file has valid Go syntax using the Go parser
func (v *GoFileValidator) hasValidGoSyntax(filePath string) (bool, error) {
	// Use Go's parser to check syntax
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)

	if err != nil {
		// Check if it's a parsing error due to incomplete file
		if strings.Contains(err.Error(), "expected") ||
			strings.Contains(err.Error(), "found 'EOF'") ||
			strings.Contains(err.Error(), "unexpected") {
			return false, nil // Invalid syntax, but not a system error
		}
		return false, err // Other errors (file access, etc.)
	}

	return true, nil
}

// HasMinimumGoContent checks if file has at least a package declaration
func (v *GoFileValidator) HasMinimumGoContent(filePath string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") {
			continue
		}

		// Check if line starts with package declaration
		if strings.HasPrefix(line, "package ") {
			return true, nil
		}

		// If we find any other non-comment content before package declaration,
		// it's not a valid Go file
		return false, nil
	}

	return false, scanner.Err()
}

// IsFileBeingWritten tries to detect if a file is currently being written
// by checking for incomplete content patterns
func (v *GoFileValidator) IsFileBeingWritten(filePath string) (bool, error) {
	hasValidSyntax, err := v.hasValidGoSyntax(filePath)
	if err != nil {
		return false, err
	}

	// If syntax is invalid, check if it looks like it's being written
	if !hasValidSyntax {
		hasMinContent, err := v.HasMinimumGoContent(filePath)
		if err != nil {
			return false, err
		}

		// If it has some content but invalid syntax, likely being written
		if !hasMinContent {
			info, err := os.Stat(filePath)
			if err != nil {
				return false, err
			}
			// If file has some content but no package declaration, likely being written
			return info.Size() > 0, nil
		}
	}

	return false, nil
}
