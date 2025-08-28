package godepfind

import (
	"fmt"
	"path/filepath"
	"strings"
)

// DebugThisFileIsMine provides detailed debugging for production issues
// with ThisFileIsMine returning unexpected results
func (g *GoDepFind) DebugThisFileIsMine(mainInputFileRelativePath, fileAbsPath, event string) (bool, error) {
	var log strings.Builder

	log.WriteString("=== DEBUG ThisFileIsMine ===\n")
	log.WriteString("1) Input parameters:\n")
	log.WriteString(fmt.Sprintf("   - mainInputFileRelativePath: %s\n", mainInputFileRelativePath))
	log.WriteString(fmt.Sprintf("   - fileAbsPath: %s\n", fileAbsPath))
	log.WriteString(fmt.Sprintf("   - event: %s\n", event))
	log.WriteString(fmt.Sprintf("   - rootDir: %s\n", g.rootDir))

	// Check cache state BEFORE initialization
	log.WriteString("2) Cache state BEFORE initialization:\n")
	log.WriteString(fmt.Sprintf("   - cachedModule: %v\n", g.cachedModule))
	log.WriteString(fmt.Sprintf("   - mainPackages count: %d\n", len(g.mainPackages)))
	log.WriteString(fmt.Sprintf("   - packageCache count: %d\n", len(g.packageCache)))

	// Force cache initialization and show the result
	log.WriteString("3) Forcing cache initialization:\n")
	err := g.ensureCacheInitialized()
	if err != nil {
		log.WriteString(fmt.Sprintf("   - ERROR during cache initialization: %v\n", err))
		fmt.Print(log.String())
		return false, err
	}
	log.WriteString("   - Cache initialization completed successfully\n")

	// Check cache state AFTER initialization
	log.WriteString("4) Cache state AFTER initialization:\n")
	log.WriteString(fmt.Sprintf("   - cachedModule: %v\n", g.cachedModule))
	log.WriteString(fmt.Sprintf("   - mainPackages count: %d\n", len(g.mainPackages)))
	log.WriteString(fmt.Sprintf("   - mainPackages: %v\n", g.mainPackages))
	log.WriteString(fmt.Sprintf("   - packageCache count: %d\n", len(g.packageCache)))
	log.WriteString(fmt.Sprintf("   - filePathToPackage count: %d\n", len(g.filePathToPackage)))
	log.WriteString(fmt.Sprintf("   - fileToPackages count: %d\n", len(g.fileToPackages)))

	// Normalize path like the real method does
	if fileAbsPath == "" {
		log.WriteString("fileAbsPath cannot be empty\n")
		fmt.Print(log.String())
		return false, fmt.Errorf("fileAbsPath cannot be empty")
	}

	if !filepath.IsAbs(fileAbsPath) {
		fileAbsPath = filepath.Join(g.rootDir, fileAbsPath)
	}
	absFilePath, err := filepath.Abs(fileAbsPath)
	if err != nil {
		log.WriteString(fmt.Sprintf("cannot resolve fileAbsPath to absolute path: %v\n", err))
		fmt.Print(log.String())
		return false, fmt.Errorf("cannot resolve fileAbsPath to absolute path: %w", err)
	}
	fileAbsPath = absFilePath
	fileName := filepath.Base(fileAbsPath)

	log.WriteString("5) After normalization:\n")
	log.WriteString(fmt.Sprintf("   - normalized fileAbsPath: %s\n", fileAbsPath))
	log.WriteString(fmt.Sprintf("   - fileName: %s\n", fileName))

	// Check direct file comparison
	handlerFile := mainInputFileRelativePath
	log.WriteString("6) Direct file comparison:\n")
	log.WriteString(fmt.Sprintf("   - handlerFile: %s\n", handlerFile))

	if fileAbsPath != "" && handlerFile != "" {
		handlerFileName := filepath.Base(handlerFile)
		log.WriteString(fmt.Sprintf("   - handlerFileName: %s\n", handlerFileName))
		log.WriteString(fmt.Sprintf("   - fileName == handlerFileName: %v\n", fileName == handlerFileName))

		if fileName == handlerFileName {
			relativeFilePath := strings.TrimPrefix(fileAbsPath, g.rootDir+"/")
			log.WriteString(fmt.Sprintf("   - relativeFilePath: %s\n", relativeFilePath))
			log.WriteString(fmt.Sprintf("   - relativeFilePath == handlerFile: %v\n", relativeFilePath == handlerFile))

			if relativeFilePath == handlerFile {
				// Successful match - don't print debug log
				return true, nil
			}
		}
	}

	// Check package resolution
	log.WriteString("7) Package resolution:\n")
	var targetPkg string

	// Check filePathToPackage cache
	if pkg, exists := g.filePathToPackage[fileAbsPath]; exists {
		targetPkg = pkg
		log.WriteString(fmt.Sprintf("   - found in filePathToPackage[%s]: %s\n", fileAbsPath, pkg))
	} else {
		log.WriteString(fmt.Sprintf("   - NOT found in filePathToPackage for: %s\n", fileAbsPath))
		// Show what's actually in the cache
		log.WriteString("   - filePathToPackage contents:\n")
		for path, pkg := range g.filePathToPackage {
			log.WriteString(fmt.Sprintf("     - %s -> %s\n", path, pkg))
		}
	}

	// Check fileToPackages cache
	packages := g.fileToPackages[fileName]
	log.WriteString(fmt.Sprintf("   - fileToPackages[%s]: %v\n", fileName, packages))

	if targetPkg == "" && len(packages) > 0 {
		targetPkg = packages[0]
		log.WriteString(fmt.Sprintf("   - using first package: %s\n", targetPkg))
	}

	if targetPkg == "" {
		log.WriteString("8) RESULT: false (no package found)\n")
		fmt.Print(log.String())
		return false, nil
	}

	// Check if it's a main package
	isMain := g.isMainPackage(targetPkg)
	log.WriteString("8) Package analysis:\n")
	log.WriteString(fmt.Sprintf("   - targetPkg: %s\n", targetPkg))
	log.WriteString(fmt.Sprintf("   - isMainPackage: %v\n", isMain))

	if isMain {
		matches := g.matchesHandlerFile(targetPkg, handlerFile)
		log.WriteString(fmt.Sprintf("   - matchesHandlerFile: %v\n", matches))

		// DEBUG: Let's see what's happening inside matchesHandlerFile
		log.WriteString("   - DEBUG matchesHandlerFile breakdown:\n")
		baseName := filepath.Base(targetPkg)
		handlerFileName := filepath.Base(handlerFile)
		log.WriteString(fmt.Sprintf("     - baseName (from targetPkg): %s\n", baseName))
		log.WriteString(fmt.Sprintf("     - handlerFileName: %s\n", handlerFileName))
		log.WriteString(fmt.Sprintf("     - baseName == handlerFile: %v\n", baseName == handlerFile))
		log.WriteString(fmt.Sprintf("     - baseName == handlerFileName: %v\n", baseName == handlerFileName))

		handlerBase := strings.TrimSuffix(handlerFileName, filepath.Ext(handlerFileName))
		log.WriteString(fmt.Sprintf("     - handlerBase (without extension): %s\n", handlerBase))

		if strings.Contains(handlerBase, ".") {
			parts := strings.Split(handlerBase, ".")
			log.WriteString(fmt.Sprintf("     - handlerBase parts: %v\n", parts))
			for _, part := range parts {
				if part != "main" && part != "" {
					contains := strings.Contains(targetPkg, part)
					log.WriteString(fmt.Sprintf("     - strings.Contains(%s, %s): %v\n", targetPkg, part, contains))
					if contains {
						log.WriteString(fmt.Sprintf("     - SHOULD MATCH! Found part '%s' in targetPkg\n", part))
					}
				}
			}
		}

		if matches {
			// Successful match - don't print debug log
			return true, nil
		}
	}

	// Check reverse dependencies
	log.WriteString("9) Reverse dependency analysis:\n")
	for _, mainPath := range g.mainPackages {
		imports := g.cachedMainImportsPackage(mainPath, targetPkg)
		matches := g.matchesHandlerFile(mainPath, handlerFile)
		log.WriteString(fmt.Sprintf("   - mainPath: %s, imports %s: %v, matches handler: %v\n",
			mainPath, targetPkg, imports, matches))

		if imports && matches {
			log.WriteString("10) RESULT: true (reverse dependency match)\n")
			// Successful match - don't print debug log
			return true, nil
		}
	}

	log.WriteString("10) RESULT: false (no matches found)\n")
	fmt.Print(log.String())
	return false, nil
}
