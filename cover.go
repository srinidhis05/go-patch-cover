package patchcover

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"golang.org/x/tools/cover"
)

// Line Struct to store information about covered and uncovered lines
type Line struct {
	LineNum    int
	NumStmt    int
	CoverCount int
	LineString string
}

func ProcessFiles(coverageFile, diffFile, prevCovFile string) (CoverageData, error) {
	patch, err := os.Open(diffFile)
	if err != nil {
		return CoverageData{}, err
	}

	files, _, err := gitdiff.Parse(patch)
	if err != nil {
		return CoverageData{}, err
	}

	profiles, err := cover.ParseProfiles(coverageFile)
	if err != nil {
		return CoverageData{}, err
	}

	var prevProfiles []*cover.Profile
	if prevCovFile != "" {
		prevProfiles, err = cover.ParseProfiles(prevCovFile)
		if err != nil {
			return CoverageData{}, err
		}
	}

	d, err := computeCoverage(files, profiles, prevProfiles)
	if err != nil {
		return CoverageData{}, err
	}

	d.HasPrevCoverage = prevCovFile != ""
	return d, nil
}

type CoverageData struct {
	NumStmt         int     `json:"num_stmt"`
	CoverCount      int     `json:"cover_count"`
	Coverage        float64 `json:"coverage"`
	PatchNumStmt    int     `json:"patch_num_stmt"`
	PatchCoverCount int     `json:"patch_cover_count"`
	PatchCoverage   float64 `json:"patch_coverage"`
	HasPrevCoverage bool    `json:"has_prev_coverage"`
	PrevNumStmt     int     `json:"prev_num_stmt"`
	PrevCoverCount  int     `json:"prev_cover_count"`
	PrevCoverage    float64 `json:"prev_coverage"`
	Uncovered_lines string  `json:"uncovered_lines"`
}

func RenderTemplateOutput(data CoverageData, tmplOverride string, out io.Writer) error {
	const defaultTmpl = `
{{- if .HasPrevCoverage -}}
	previous coverage: {{printf "%.1f" .PrevCoverage}}% of statements
{{ else -}}
	previous coverage: unknown
{{ end -}}
new coverage: {{printf "%.1f" .Coverage}}% of statements
patch coverage: {{printf "%.1f" .PatchCoverage}}% of changed statements ({{ .PatchCoverCount }}/{{ .PatchNumStmt }})
uncovered lines : {{printf .Uncovered_lines }}
`
	tmpl := defaultTmpl
	if tmplOverride != "" {
		tmpl = tmplOverride
	}
	t, err := template.New("cover_template").Parse(tmpl)
	if err != nil {
		return err
	}
	return t.ExecuteTemplate(out, "cover_template", data)
}

func computeCoverage(diffFiles []*gitdiff.File, coverProfiles []*cover.Profile, prevCoverProfiles []*cover.Profile) (CoverageData, error) {
	var data CoverageData
	coveredLines := make(map[string][]Line)
	partiallyCoveredLines := make(map[string][]Line)

	// patch coverage
	for _, p := range coverProfiles {
		for _, f := range diffFiles {
			// Using suffix since profiles are prepended with the go module.
			if !strings.HasSuffix(p.FileName, f.NewName) {
				//fmt.Printf("%s != %s\n", p.FileName, f.NewName)
				continue
			}

		blockloop:
			for _, b := range p.Blocks {
				//fmt.Printf("BLOCK %s:%d %d %d %d\n", p.FileName, b.StartLine, b.EndLine, b.NumStmt, b.Count)
				for _, t := range f.TextFragments {
					for i, line := range t.Lines {
						if line.Op != gitdiff.OpAdd {
							continue
						}
						lineNum := int(t.NewPosition) + i
						lineString := strings.ReplaceAll(line.Line, "\n", "")
						//fmt.Printf("DIFF %s:%d %s\n", f.NewName, lineNum, lineString)

						if b.StartLine <= lineNum && lineNum <= b.EndLine {
							data.PatchNumStmt += b.NumStmt
							//	fmt.Printf("COVER %s:%d %d %d - %s\n", p.FileName, lineNum, b.NumStmt, b.Count, lineString)
							if b.Count > 0 {
								data.PatchCoverCount += b.NumStmt
								// Line covered
								coveredLines[p.FileName] = append(coveredLines[p.FileName], Line{
									LineNum:    lineNum,
									NumStmt:    b.NumStmt,
									CoverCount: b.Count,
									LineString: lineString,
								})
							} else {
								// Line not covered (or) partially covered
								partiallyCoveredLines[p.FileName] = append(partiallyCoveredLines[p.FileName], Line{
									LineNum:    lineNum,
									NumStmt:    b.NumStmt,
									CoverCount: b.Count,
									LineString: lineString,
								})
							}
							continue blockloop
						}
					}
				}
			}
		}
	}

	// total coverage
	for _, p := range coverProfiles {
		for _, b := range p.Blocks {
			data.NumStmt += b.NumStmt
			if b.Count > 0 {
				data.CoverCount += b.NumStmt
			}
		}
	}

	// prev total coverage
	for _, p := range prevCoverProfiles {
		for _, b := range p.Blocks {
			data.PrevNumStmt += b.NumStmt
			if b.Count > 0 {
				data.PrevCoverCount += b.NumStmt
			}
		}
	}

	// Get uncovered lines and write to the file
	data = printUncoveredLines(partiallyCoveredLines, coveredLines, data)

	if data.NumStmt != 0 {
		data.Coverage = float64(data.CoverCount) / float64(data.NumStmt) * 100
	}
	if data.PatchNumStmt != 0 {
		data.PatchCoverage = float64(data.PatchCoverCount) / float64(data.PatchNumStmt) * 100
	}
	if data.PrevNumStmt != 0 {
		data.PrevCoverage = float64(data.PrevCoverCount) / float64(data.PrevNumStmt) * 100
	}

	//Condition to set coverage to 100% if no lines added in PR
	if data.PatchNumStmt == 0 {
		data.PatchCoverage = 100.0
	}

	return data, nil
}

/*
The lines which are partially covered but not inside coveredLines are the uncovered lines. after we filter those lines,
we print these lines to uncovered_lines.txt. For these invalid lines, we modify patch coverage in following way:
For valid covered line - Don't change patch coverage
For valid uncovered line - Don't change patch coverage
For Invalid covered line - subtract PatchNumStmt
For Invalid uncovered line - subtract PatchNumStmt, PatchCoverCount
*/
func printUncoveredLines(partiallyCoveredLines, coveredLines map[string][]Line, data CoverageData) CoverageData {
	// Open a new file for writing
	file, err := os.Create("uncovered_lines.txt")
	if err != nil {
		fmt.Println("Error creating file:", err)
	}
	defer file.Close()

	// Get uncovered lines and write to the file
	for fileName, lines := range partiallyCoveredLines {
		// Check if the file is covered
		_, ok := coveredLines[fileName]

		// Create a slice to store uncovered lines to keep
		var uncoveredLines []Line

		for _, line := range lines {
			// Check if line is a comment, empty, or a new line without code
			uncovered := !ok || !isLineCovered(line, coveredLines[fileName])

			if !isInvalidLine(line.LineString) {
				if uncovered {
					uncoveredLines = append(uncoveredLines, line)
				}
			} else {
				data.PatchNumStmt -= line.NumStmt
				if !uncovered {
					data.PatchCoverCount -= line.NumStmt
				}
			}
		}

		// Write to the file if there are any remaining-uncovered lines
		if len(uncoveredLines) > 0 {
			// Write the filename to the file
			file.WriteString("<pre>\n")
			file.WriteString(fmt.Sprintf("Uncovered lines in %s:\n", fileName))

			for _, line := range uncoveredLines {
				// Write the line number to the file
				file.WriteString(fmt.Sprintf("LineNum: %d\n", line.LineNum))
				// Write the line string to the file
				file.WriteString(fmt.Sprintf("Lines:\n <code>%s</code>\n", line.LineString))
			}

			// Write a separator to separate the sections for different files
			file.WriteString("\n-----------------------\n")
			file.WriteString("</pre>\n")
		}
	}

	content, err := os.ReadFile("uncovered_lines.txt")
	if err != nil {
		fmt.Println("Error reading file:", err)
	}
	fmt.Println(string(content))
	fmt.Println("Uncovered lines have been saved to uncovered_lines.txt.")
	data.Uncovered_lines = string(content)
	return data
}

// comments, and structs are excluded from uncovered lines
func isInvalidLine(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || strings.HasSuffix(line, "*/") || line == "" || strings.Contains(line, "`json:")
}

func isLineCovered(line Line, coveredLines []Line) bool {
	for _, coveredLine := range coveredLines {
		if coveredLine.LineNum == line.LineNum && coveredLine.LineString == line.LineString && coveredLine.CoverCount == line.CoverCount {
			return true
		}
	}
	return false
}
