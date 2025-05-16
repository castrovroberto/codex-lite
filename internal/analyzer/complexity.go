package analyzer

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ComplexityInfo holds code complexity metrics
type ComplexityInfo struct {
	Path       string // File path
	Functions  []FuncComplexity
	TotalLines int     // Total lines in file
	AvgCCN     float64 // Average cyclomatic complexity
}

// FuncComplexity holds complexity metrics for a single function
type FuncComplexity struct {
	Name     string
	CCN      int     // Cyclomatic Complexity Number
	Lines    int     // Lines of code
	Nesting  int     // Maximum nesting level
	DocScore float64 // Documentation score (0-1)
}

// AnalyzeComplexity performs code complexity analysis
func AnalyzeComplexity(rootPath string) ([]*ComplexityInfo, error) {
	var results []*ComplexityInfo

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if IsSkippableDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// Currently only supporting Go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		info, err := analyzeGoFile(path)
		if err != nil {
			return fmt.Errorf("failed to analyze %s: %w", path, err)
		}

		if info != nil {
			results = append(results, info)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to analyze complexity: %w", err)
	}

	return results, nil
}

func analyzeGoFile(path string) (*ComplexityInfo, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	info := &ComplexityInfo{
		Path:       path,
		TotalLines: bytes.Count(content, []byte{'\n'}) + 1,
	}

	var totalCCN int
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			complexity := analyzeFuncComplexity(node, fset)
			info.Functions = append(info.Functions, complexity)
			totalCCN += complexity.CCN
		}
		return true
	})

	if len(info.Functions) > 0 {
		info.AvgCCN = float64(totalCCN) / float64(len(info.Functions))
	}

	return info, nil
}

func analyzeFuncComplexity(fn *ast.FuncDecl, fset *token.FileSet) FuncComplexity {
	complexity := FuncComplexity{
		Name: fn.Name.Name,
		CCN:  1, // Base complexity
	}

	// Calculate lines
	if fn.Body != nil {
		start := fset.Position(fn.Pos())
		end := fset.Position(fn.End())
		complexity.Lines = end.Line - start.Line + 1
	}

	// Calculate documentation score
	if fn.Doc != nil {
		docLines := len(fn.Doc.List)
		complexity.DocScore = float64(docLines) / float64(complexity.Lines)
		if complexity.DocScore > 1 {
			complexity.DocScore = 1
		}
	}

	// Calculate CCN and nesting
	ast.Inspect(fn, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause,
			*ast.CommClause, *ast.BinaryExpr:
			complexity.CCN++
		}
		return true
	})

	// Calculate maximum nesting level
	complexity.Nesting = calculateNesting(fn)

	return complexity
}

func calculateNesting(fn *ast.FuncDecl) int {
	maxDepth := 0
	currentDepth := 0

	ast.Inspect(fn, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SelectStmt, *ast.SwitchStmt:
			currentDepth++
			if currentDepth > maxDepth {
				maxDepth = currentDepth
			}
		}
		return true
	})

	return maxDepth
}

// FormatComplexityAnalysis returns a human-readable summary of code complexity
func FormatComplexityAnalysis(results []*ComplexityInfo) string {
	var b strings.Builder

	if len(results) == 0 {
		b.WriteString("🔍 No files analyzed for complexity\n")
		return b.String()
	}

	b.WriteString("🔍 Code Complexity Analysis:\n\n")

	var totalCCN float64
	var totalFiles int
	var complexFuncs []string

	for _, info := range results {
		relPath, err := filepath.Rel(".", info.Path)
		if err != nil {
			relPath = info.Path
		}

		b.WriteString(fmt.Sprintf("📄 %s:\n", relPath))
		b.WriteString(fmt.Sprintf("  Lines: %d, Avg Complexity: %.2f\n", info.TotalLines, info.AvgCCN))

		// List complex functions (CCN > 10)
		for _, fn := range info.Functions {
			if fn.CCN > 10 {
				complexFuncs = append(complexFuncs,
					fmt.Sprintf("  - %s (CCN: %d, Lines: %d, Nesting: %d, Doc: %.0f%%)\n",
						fn.Name, fn.CCN, fn.Lines, fn.Nesting, fn.DocScore*100))
			}
		}

		totalCCN += info.AvgCCN
		totalFiles++
		b.WriteString("\n")
	}

	// Overall statistics
	b.WriteString("📊 Overall Statistics:\n")
	b.WriteString(fmt.Sprintf("  Files Analyzed: %d\n", totalFiles))
	b.WriteString(fmt.Sprintf("  Average Complexity: %.2f\n", totalCCN/float64(totalFiles)))

	// List complex functions
	if len(complexFuncs) > 0 {
		b.WriteString("\n⚠️ Complex Functions (CCN > 10):\n")
		for _, fn := range complexFuncs {
			b.WriteString(fn)
		}
	}

	return b.String()
}
