package converter

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/500wpm/backend/internal/models"
)

// Supported file formats
const (
	FormatTXT  = "txt"
	FormatEPUB = "epub"
	FormatMOBI = "mobi"
	FormatText = "text" // For pasted text (not a file)
)

// Common errors
var (
	ErrUnsupportedFormat = errors.New("unsupported file format")
	ErrEmptyContent      = errors.New("file content is empty")
	ErrInvalidFile       = errors.New("invalid or corrupted file")
)

// Converter interface for all format converters
type Converter interface {
	// Convert takes file content and returns parsed book content
	Convert(content []byte, filename string) (*models.BookContent, error)
	// SupportedFormat returns the format this converter handles
	SupportedFormat() string
}

// Manager handles format detection and conversion routing
type Manager struct {
	converters map[string]Converter
}

// NewManager creates a new converter manager with all supported converters
func NewManager() *Manager {
	m := &Manager{
		converters: make(map[string]Converter),
	}

	// Register all converters
	m.Register(&TXTConverter{})
	m.Register(&EPUBConverter{})
	m.Register(&MOBIConverter{})

	return m
}

// Register adds a converter for a specific format
func (m *Manager) Register(c Converter) {
	m.converters[c.SupportedFormat()] = c
}

// DetectFormat detects the file format from filename and/or content
func (m *Manager) DetectFormat(filename string, content []byte) string {
	// First try extension
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".txt":
		return FormatTXT
	case ".epub":
		return FormatEPUB
	case ".mobi", ".azw", ".azw3", ".prc":
		return FormatMOBI
	}

	// Try magic bytes detection
	if len(content) >= 4 {
		// EPUB is a ZIP file starting with PK
		if content[0] == 0x50 && content[1] == 0x4B {
			return FormatEPUB
		}
		// MOBI/PRC starts with specific header at offset 60
		if len(content) >= 68 {
			magic := string(content[60:68])
			if magic == "BOOKMOBI" || magic == "TEXtREAd" {
				return FormatMOBI
			}
		}
	}

	// Default to TXT for unknown formats
	return FormatTXT
}

// Convert converts file content to BookContent
func (m *Manager) Convert(content []byte, filename string) (*models.BookContent, error) {
	if len(content) == 0 {
		return nil, ErrEmptyContent
	}

	format := m.DetectFormat(filename, content)
	converter, ok := m.converters[format]
	if !ok {
		return nil, ErrUnsupportedFormat
	}

	return converter.Convert(content, filename)
}

// ConvertText converts plain text (from paste) to BookContent
func (m *Manager) ConvertText(text, title string) (*models.BookContent, error) {
	if len(strings.TrimSpace(text)) == 0 {
		return nil, ErrEmptyContent
	}

	normalizedText := models.NormalizeText(text)

	if title == "" {
		title = "Pasted Text"
	}

	chapters := []models.Chapter{
		{
			Title:   title,
			Content: normalizedText,
		},
	}

	return models.NewBookContent(title, "", "", FormatText, chapters), nil
}

// SupportedFormats returns a list of supported file extensions
func (m *Manager) SupportedFormats() []string {
	return []string{".txt", ".epub", ".mobi", ".azw", ".azw3", ".prc"}
}
