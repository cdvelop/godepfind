package godepfind

// Example of how ValidateInputForProcessing can be reused in other public APIs

// FindReverseDepsForFile finds reverse dependencies for a specific file with validation
// This is an example of how to reuse the validation function in other public APIs
func (g *GoDepFind) FindReverseDepsForFile(mainFilePath, fileName, filePath string) ([]string, error) {
	// Reuse centralized validation
	shouldProcess, err := g.ValidateInputForProcessing(mainFilePath, fileName, filePath)
	if err != nil {
		return nil, err
	}
	if !shouldProcess {
		return []string{}, nil // Return empty result for invalid/incomplete files
	}

	// Find which package contains the file
	pkg, err := g.findPackageContainingFile(fileName)
	if err != nil {
		return nil, err
	}
	if pkg == "" {
		return []string{}, nil // File not found in any package
	}

	// Find reverse dependencies
	return g.FindReverseDeps("./...", []string{pkg})
}

// CheckFileOwnership checks if a file belongs to a handler with validation
// Another example of reusing the validation function
func (g *GoDepFind) CheckFileOwnership(mainFilePath, fileName, filePath string) (string, error) {
	// Reuse centralized validation
	shouldProcess, err := g.ValidateInputForProcessing(mainFilePath, fileName, filePath)
	if err != nil {
		return "", err
	}
	if !shouldProcess {
		return "skipped", nil // File is being written or invalid
	}

	// Check ownership using existing logic
	belongs, err := g.ThisFileIsMine(mainFilePath, filePath, "check")
	if err != nil {
		return "", err
	}

	if belongs {
		return "owned", nil
	}
	return "not-owned", nil
}

// AnalyzeFileImpact analyzes the impact of a file change with validation
// Yet another example showing reusability
func (g *GoDepFind) AnalyzeFileImpact(mainFilePath, fileName, filePath, event string) (*FileImpactResult, error) {
	// Reuse centralized validation
	shouldProcess, err := g.ValidateInputForProcessing(mainFilePath, fileName, filePath)
	if err != nil {
		return nil, err
	}
	if !shouldProcess {
		return &FileImpactResult{
			Status: "skipped",
			Reason: "File is invalid, empty, or being written",
			Impact: "none",
		}, nil
	}

	// Perform impact analysis
	mainPackages, err := g.GoFileComesFromMain(fileName)
	if err != nil {
		return nil, err
	}

	belongs, err := g.ThisFileIsMine(mainFilePath, filePath, event)
	if err != nil {
		return nil, err
	}

	return &FileImpactResult{
		Status:           "analyzed",
		BelongsToHandler: belongs,
		AffectedMains:    mainPackages,
		Impact:           calculateImpact(len(mainPackages), belongs),
	}, nil
}

// FileImpactResult represents the result of file impact analysis
type FileImpactResult struct {
	Status           string   `json:"status"`
	Reason           string   `json:"reason,omitempty"`
	BelongsToHandler bool     `json:"belongs_to_handler"`
	AffectedMains    []string `json:"affected_mains"`
	Impact           string   `json:"impact"`
}

// calculateImpact determines the impact level based on analysis results
func calculateImpact(mainCount int, belongsToHandler bool) string {
	if mainCount == 0 {
		return "none"
	}
	if belongsToHandler && mainCount == 1 {
		return "low"
	}
	if belongsToHandler && mainCount > 1 {
		return "medium"
	}
	if !belongsToHandler && mainCount > 0 {
		return "high" // File affects mains but doesn't belong to current handler
	}
	return "unknown"
}
