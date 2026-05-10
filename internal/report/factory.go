package report

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
)

// ReportersFromFormats creates a slice of reporters based on format names.
// Supported formats: "cli", "json", "html", "junit".
func ReportersFromFormats(formats []string, outputDir string) ([]Reporter, error) {
	var reporters []Reporter

	for _, format := range formats {
		r, err := NewReporterFromFormat(format, outputDir)
		if err != nil {
			return nil, err
		}
		reporters = append(reporters, r)
	}

	return reporters, nil
}

// NewReporterFromFormat creates a single reporter for the given format.
func NewReporterFromFormat(format, outputDir string) (Reporter, error) {
	switch format {
	case "cli":
		return NewCLIReporter(os.Stdout), nil

	case "json":
		path := filepath.Join(outputDir, "report.json")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return nil, fmt.Errorf("creating output dir: %w", err)
		}
		f, err := os.Create(path)
		if err != nil {
			return nil, fmt.Errorf("creating JSON report file: %w", err)
		}
		return NewJSONReporter(f), nil

	case "html":
		path := filepath.Join(outputDir, "report.html")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return nil, fmt.Errorf("creating output dir: %w", err)
		}
		tmpl := loadHTMLTemplate()
		return NewHTMLReporter(path, tmpl), nil

	case "junit":
		path := filepath.Join(outputDir, "report.xml")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return nil, fmt.Errorf("creating output dir: %w", err)
		}
		f, err := os.Create(path)
		if err != nil {
			return nil, fmt.Errorf("creating JUnit report file: %w", err)
		}
		return NewJUnitReporter(f), nil

	default:
		return nil, fmt.Errorf("unsupported report format: %q", format)
	}
}

func loadHTMLTemplate() *template.Template {
	tmplPath := "templates/report.html"
	if _, err := os.Stat(tmplPath); err == nil {
		tmpl, err := template.ParseFiles(tmplPath)
		if err == nil {
			return tmpl
		}
	}
	return defaultTemplate()
}
