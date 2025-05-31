package analyzer

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/castrovroberto/CGE/internal/security"
)

// SecurityIssue represents a potential security concern
type SecurityIssue struct {
	Path        string
	Line        int
	Type        string
	Description string
	Severity    string
	Context     string
}

// Common security patterns to check
var securityPatterns = []struct {
	Type        string
	Pattern     *regexp.Regexp
	Description string
	Severity    string
}{
	{
		Type:        "API Key",
		Pattern:     regexp.MustCompile(`(?i)(api[_-]?key|apikey|secret|token)["\s]*[:=]\s*["']?[A-Za-z0-9+/=]{32,}["']?`),
		Description: "Possible hardcoded API key or secret",
		Severity:    "HIGH",
	},
	{
		Type:        "Password",
		Pattern:     regexp.MustCompile(`(?i)(password|passwd|pwd)["\s]*[:=]\s*["'][^"']{8,}["']`),
		Description: "Possible hardcoded password",
		Severity:    "HIGH",
	},
	{
		Type:        "Private Key",
		Pattern:     regexp.MustCompile(`-{5}BEGIN [A-Z]+ PRIVATE KEY-{5}`),
		Description: "Private key found in source code",
		Severity:    "CRITICAL",
	},
	{
		Type:        "SQL Injection",
		Pattern:     regexp.MustCompile(`(?i)(SELECT|INSERT|UPDATE|DELETE).*\+\s*['"]\s*\+`),
		Description: "Potential SQL injection vulnerability",
		Severity:    "HIGH",
	},
	{
		Type:        "Command Injection",
		Pattern:     regexp.MustCompile(`(?i)(exec|spawn|system)\s*\([^)]*\$`),
		Description: "Potential command injection vulnerability",
		Severity:    "HIGH",
	},
	{
		Type:        "Insecure Hash",
		Pattern:     regexp.MustCompile(`(?i)(md5|sha1)\(`),
		Description: "Use of cryptographically insecure hash function",
		Severity:    "MEDIUM",
	},
	{
		Type:        "Debug Mode",
		Pattern:     regexp.MustCompile(`(?i)(debug|development)\s*[=:]\s*true`),
		Description: "Debug/development mode enabled",
		Severity:    "LOW",
	},
}

// Common sensitive file patterns
var sensitiveFiles = []struct {
	Pattern     string
	Description string
	Severity    string
}{
	{".env", "Environment file may contain secrets", "HIGH"},
	{"id_rsa", "SSH private key", "HIGH"},
	{".pem", "SSL/TLS private key", "HIGH"},
	{".pfx", "SSL/TLS key store", "HIGH"},
	{".key", "Cryptographic key file", "HIGH"},
	{"secrets.yaml", "Kubernetes secrets file", "HIGH"},
	{"credentials.json", "Credentials file", "HIGH"},
}

// AnalyzeSecurity performs security analysis of the codebase
func AnalyzeSecurity(rootPath string) ([]SecurityIssue, error) {
	// Create safe file operations with root path as allowed root
	safeOps := security.NewSafeFileOps(rootPath)

	var issues []SecurityIssue

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

		// Check for sensitive files
		baseName := strings.ToLower(d.Name())
		for _, sf := range sensitiveFiles {
			if strings.Contains(baseName, sf.Pattern) {
				issues = append(issues, SecurityIssue{
					Path:        path,
					Type:        "Sensitive File",
					Description: sf.Description,
					Severity:    sf.Severity,
				})
			}
		}

		// Skip binary and large files
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Size() > 1024*1024 { // Skip files larger than 1MB
			return nil
		}

		// Read and analyze file content using secure file operations
		content, err := safeOps.SafeReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			for _, pattern := range securityPatterns {
				if pattern.Pattern.MatchString(line) {
					// Get context (line before and after)
					start := max(0, i-1)
					end := min(len(lines), i+2)
					context := strings.Join(lines[start:end], "\n")

					issues = append(issues, SecurityIssue{
						Path:        path,
						Line:        i + 1,
						Type:        pattern.Type,
						Description: pattern.Description,
						Severity:    pattern.Severity,
						Context:     context,
					})
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to analyze security: %w", err)
	}

	return issues, nil
}

// FormatSecurityAnalysis returns a human-readable summary of security issues
func FormatSecurityAnalysis(issues []SecurityIssue) string {
	var b strings.Builder

	if len(issues) == 0 {
		b.WriteString("🔒 No security issues found\n")
		return b.String()
	}

	b.WriteString("⚠️ Security Analysis Results:\n\n")

	// Group issues by severity
	severityGroups := make(map[string][]SecurityIssue)
	for _, issue := range issues {
		severityGroups[issue.Severity] = append(severityGroups[issue.Severity], issue)
	}

	// Print issues by severity (CRITICAL -> HIGH -> MEDIUM -> LOW)
	for _, severity := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"} {
		issues := severityGroups[severity]
		if len(issues) == 0 {
			continue
		}

		b.WriteString(fmt.Sprintf("🚨 %s Severity Issues:\n", severity))
		for _, issue := range issues {
			relPath, err := filepath.Rel(".", issue.Path)
			if err != nil {
				relPath = issue.Path
			}

			b.WriteString(fmt.Sprintf("\n  📄 %s", relPath))
			if issue.Line > 0 {
				b.WriteString(fmt.Sprintf(":%d", issue.Line))
			}
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("  Type: %s\n", issue.Type))
			b.WriteString(fmt.Sprintf("  Description: %s\n", issue.Description))

			if issue.Context != "" {
				b.WriteString("  Context:\n")
				for _, contextLine := range strings.Split(issue.Context, "\n") {
					b.WriteString(fmt.Sprintf("    %s\n", contextLine))
				}
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}
