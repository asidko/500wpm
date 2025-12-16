package converter

import (
	"path/filepath"
	"strings"

	"github.com/500wpm/backend/internal/models"
)

// TXTConverter handles plain text files
type TXTConverter struct{}

// SupportedFormat returns the format this converter handles
func (c *TXTConverter) SupportedFormat() string {
	return FormatTXT
}

// Convert parses a TXT file into BookContent
func (c *TXTConverter) Convert(content []byte, filename string) (*models.BookContent, error) {
	text := string(content)

	if len(strings.TrimSpace(text)) == 0 {
		return nil, ErrEmptyContent
	}

	// Normalize the text
	text = models.NormalizeText(text)

	// Extract title from filename
	title := extractTitle(filename)

	// For TXT files, create a single chapter
	chapters := []models.Chapter{
		{
			Title:   title,
			Content: text,
		},
	}

	return models.NewBookContent(title, "", "", FormatTXT, chapters), nil
}

// extractTitle extracts a clean title from a filename
func extractTitle(filename string) string {
	if filename == "" {
		return "Untitled"
	}

	// Remove extension
	title := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))

	// Replace common separators with spaces
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.ReplaceAll(title, "-", " ")

	// Clean up multiple spaces
	for strings.Contains(title, "  ") {
		title = strings.ReplaceAll(title, "  ", " ")
	}

	title = strings.TrimSpace(title)

	if title == "" {
		return "Untitled"
	}

	return title
}
