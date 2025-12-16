package models

import (
	"encoding/json"
	"strings"
	"unicode"
)

// BookContent represents the full structure stored in the database as JSON
type BookContent struct {
	Version  string   `json:"version"`
	Metadata Metadata `json:"metadata"`
	Chapters []Chapter `json:"chapters"`
}

// Metadata contains book information extracted from the source file
type Metadata struct {
	Title         string `json:"title"`
	Author        string `json:"author,omitempty"`
	Language      string `json:"language,omitempty"`
	SourceFormat  string `json:"source_format"`
	TotalWords    int    `json:"total_words"`
	TotalChapters int    `json:"total_chapters"`
}

// Chapter represents a single chapter in the book
type Chapter struct {
	Index     int    `json:"index"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	WordCount int    `json:"word_count"`
}

// CurrentVersion is the JSON schema version
const CurrentVersion = "1.0"

// NewBookContent creates a new BookContent with proper initialization
func NewBookContent(title, author, language, sourceFormat string, chapters []Chapter) *BookContent {
	totalWords := 0
	for i := range chapters {
		chapters[i].Index = i
		chapters[i].WordCount = countWords(chapters[i].Content)
		totalWords += chapters[i].WordCount
	}

	return &BookContent{
		Version: CurrentVersion,
		Metadata: Metadata{
			Title:         title,
			Author:        author,
			Language:      language,
			SourceFormat:  sourceFormat,
			TotalWords:    totalWords,
			TotalChapters: len(chapters),
		},
		Chapters: chapters,
	}
}

// ToJSON serializes the book content to JSON
func (b *BookContent) ToJSON() (string, error) {
	data, err := json.Marshal(b)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ParseBookContent deserializes JSON to BookContent
func ParseBookContent(jsonStr string) (*BookContent, error) {
	var book BookContent
	if err := json.Unmarshal([]byte(jsonStr), &book); err != nil {
		return nil, err
	}
	return &book, nil
}

// countWords counts words in a text string
func countWords(text string) int {
	if len(text) == 0 {
		return 0
	}

	count := 0
	inWord := false

	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			if inWord {
				count++
				inWord = false
			}
		} else {
			inWord = true
		}
	}

	// Count last word if text doesn't end with space
	if inWord {
		count++
	}

	return count
}

// NormalizeText cleans up text content
func NormalizeText(text string) string {
	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Collapse multiple newlines to max 2
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	// Trim leading/trailing whitespace
	text = strings.TrimSpace(text)

	return text
}
