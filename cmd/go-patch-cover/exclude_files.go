package main

import (
	"bufio"
	"fmt"
	"go-patch-cover/utility"
	"os"
	"regexp"
	"strings"
)

func shouldExcludeFile(excludedPatterns, filePath string) bool {
	patterns := strings.Split(excludedPatterns, ",")
	prefix, repoName := getRepoName()
	for _, pattern := range patterns {
		pattern = prefix + repoName + "/" + pattern
		if matchesPattern(pattern, filePath) {
			return true
		}
	}
	return false
}

func getRepoName() (string, string) {
	prefix := "github.com/org/"
	switch utility.RepoName {
	case "demo-repo":
		return prefix, "demo_repo"
	default:
		return prefix, utility.RepoName
	}
}

// matchesPattern uses custom logic to check if a certain file is matching the given pattern
// Rules:
// 1. if pattern contains ** it means 0 or more directories before the expression ex: **/mock
// 2. if pattern contains * it means 0 or more characters (excluding '/') ex: *_test.go
// 3. if file contains "easyjson", exclude it

func matchesPattern(pattern, file string) bool {
	if strings.Contains(file, "easyjson") {
		return true
	}

	regexPattern := "^" + strings.ReplaceAll(pattern, "*", ".*") + "$"
	matched, _ := regexp.MatchString(regexPattern, file)

	return matched
}

func modifyCoverageFile(covFile, excludedPatterns string) error {
	content, err := os.ReadFile(covFile)
	if err != nil {
		return err
	}

	fmt.Println("******************************************************")
	// Filter out lines that match any of the exclusion patterns
	var filteredLines []string
	var filePath string
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			re := regexp.MustCompile(`^([^:]+):`)
			match := re.FindStringSubmatch(line)
			if len(match) > 1 {
				filePath = match[1]
			} else {
				continue
			}
			if !shouldExcludeFile(excludedPatterns, filePath) {
				filteredLines = append(filteredLines, line)
			} else {
				fmt.Println(line)
			}
		}
	}
	fmt.Println("******************************************************")

	if err := scanner.Err(); err != nil {
		return err
	}

	// Join the lines back to form the modified coverage content
	modifiedContent := strings.Join(filteredLines, "\n")

	// Write the modified content back to the coverage file
	err = os.WriteFile(covFile, []byte(modifiedContent), 0644)
	if err != nil {
		return err
	}

	return nil
}
