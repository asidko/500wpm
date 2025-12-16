package converter

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/net/html"

	"github.com/500wpm/backend/internal/models"
)

// EPUBConverter handles EPUB files
type EPUBConverter struct{}

// SupportedFormat returns the format this converter handles
func (c *EPUBConverter) SupportedFormat() string {
	return FormatEPUB
}

// EPUB XML structures
type container struct {
	Rootfiles []rootfile `xml:"rootfiles>rootfile"`
}

type rootfile struct {
	FullPath  string `xml:"full-path,attr"`
	MediaType string `xml:"media-type,attr"`
}

type opfPackage struct {
	Metadata opfMetadata `xml:"metadata"`
	Manifest opfManifest `xml:"manifest"`
	Spine    opfSpine    `xml:"spine"`
}

type opfMetadata struct {
	Title    string `xml:"title"`
	Creator  string `xml:"creator"`
	Language string `xml:"language"`
}

type opfManifest struct {
	Items []opfItem `xml:"item"`
}

type opfItem struct {
	ID        string `xml:"id,attr"`
	Href      string `xml:"href,attr"`
	MediaType string `xml:"media-type,attr"`
}

type opfSpine struct {
	ItemRefs []opfItemRef `xml:"itemref"`
}

type opfItemRef struct {
	IDRef  string `xml:"idref,attr"`
	Linear string `xml:"linear,attr"`
}

// ncxNavMap for TOC navigation
type ncxDocument struct {
	NavMap ncxNavMap `xml:"navMap"`
}

type ncxNavMap struct {
	NavPoints []ncxNavPoint `xml:"navPoint"`
}

type ncxNavPoint struct {
	NavLabel  ncxNavLabel `xml:"navLabel"`
	Content   ncxContent  `xml:"content"`
	PlayOrder int         `xml:"playOrder,attr"`
}

type ncxNavLabel struct {
	Text string `xml:"text"`
}

type ncxContent struct {
	Src string `xml:"src,attr"`
}

// Convert parses an EPUB file into BookContent
func (c *EPUBConverter) Convert(content []byte, filename string) (*models.BookContent, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidFile, err)
	}

	// Read container.xml to find the root file
	containerData, err := readZipFile(reader, "META-INF/container.xml")
	if err != nil {
		return nil, fmt.Errorf("failed to read container.xml: %w", err)
	}

	var cont container
	if err := xml.Unmarshal(containerData, &cont); err != nil {
		return nil, fmt.Errorf("failed to parse container.xml: %w", err)
	}

	if len(cont.Rootfiles) == 0 {
		return nil, fmt.Errorf("%w: no rootfile found", ErrInvalidFile)
	}

	rootPath := cont.Rootfiles[0].FullPath
	rootDir := path.Dir(rootPath)

	// Read and parse the OPF file
	opfData, err := readZipFile(reader, rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read OPF file: %w", err)
	}

	var pkg opfPackage
	if err := xml.Unmarshal(opfData, &pkg); err != nil {
		return nil, fmt.Errorf("failed to parse OPF file: %w", err)
	}

	// Build manifest map (id -> item)
	manifestMap := make(map[string]opfItem)
	for _, item := range pkg.Manifest.Items {
		manifestMap[item.ID] = item
	}

	// Try to read TOC for chapter titles
	tocTitles := c.readTOCTitles(reader, rootDir, &pkg)

	// Get chapters in spine order
	var chapters []models.Chapter
	chapterIndex := 0

	for _, itemRef := range pkg.Spine.ItemRefs {
		// Skip non-linear items (like cover pages)
		if itemRef.Linear == "no" {
			continue
		}

		item, ok := manifestMap[itemRef.IDRef]
		if !ok {
			continue
		}

		// Only process XHTML content
		if !isHTMLMediaType(item.MediaType) {
			continue
		}

		// Read chapter content
		chapterPath := resolvePath(rootDir, item.Href)
		chapterData, err := readZipFile(reader, chapterPath)
		if err != nil {
			continue
		}

		// Extract text from HTML
		text := extractTextFromHTML(chapterData)
		text = models.NormalizeText(text)

		// Skip empty or very short chapters (likely metadata/cover)
		if len(text) < 100 {
			continue
		}

		// Get chapter title from TOC or generate one
		title := c.getChapterTitle(tocTitles, item.Href, chapterIndex)

		chapters = append(chapters, models.Chapter{
			Title:   title,
			Content: text,
		})
		chapterIndex++
	}

	if len(chapters) == 0 {
		return nil, fmt.Errorf("%w: no readable content found", ErrInvalidFile)
	}

	// Extract title from metadata or filename
	title := pkg.Metadata.Title
	if title == "" {
		title = extractTitle(filename)
	}

	return models.NewBookContent(
		title,
		pkg.Metadata.Creator,
		pkg.Metadata.Language,
		FormatEPUB,
		chapters,
	), nil
}

// readTOCTitles attempts to read chapter titles from NCX or NAV
func (c *EPUBConverter) readTOCTitles(reader *zip.Reader, rootDir string, pkg *opfPackage) map[string]string {
	titles := make(map[string]string)

	// Find NCX file in manifest
	var ncxPath string
	for _, item := range pkg.Manifest.Items {
		if item.MediaType == "application/x-dtbncx+xml" {
			ncxPath = resolvePath(rootDir, item.Href)
			break
		}
	}

	if ncxPath == "" {
		return titles
	}

	ncxData, err := readZipFile(reader, ncxPath)
	if err != nil {
		return titles
	}

	var ncx ncxDocument
	if err := xml.Unmarshal(ncxData, &ncx); err != nil {
		return titles
	}

	// Sort by play order
	sort.Slice(ncx.NavMap.NavPoints, func(i, j int) bool {
		return ncx.NavMap.NavPoints[i].PlayOrder < ncx.NavMap.NavPoints[j].PlayOrder
	})

	for _, np := range ncx.NavMap.NavPoints {
		// Remove fragment from src (e.g., "chapter1.xhtml#start" -> "chapter1.xhtml")
		src := np.Content.Src
		if idx := strings.Index(src, "#"); idx != -1 {
			src = src[:idx]
		}
		titles[src] = np.NavLabel.Text
	}

	return titles
}

// getChapterTitle gets title from TOC or generates one
func (c *EPUBConverter) getChapterTitle(tocTitles map[string]string, href string, index int) string {
	// Try exact match
	if title, ok := tocTitles[href]; ok && title != "" {
		return title
	}

	// Try without path
	baseName := path.Base(href)
	if title, ok := tocTitles[baseName]; ok && title != "" {
		return title
	}

	// Generate title
	return fmt.Sprintf("Chapter %d", index+1)
}

// readZipFile reads a file from the ZIP archive
func readZipFile(reader *zip.Reader, path string) ([]byte, error) {
	// Normalize path separators
	path = strings.ReplaceAll(path, "\\", "/")

	for _, f := range reader.File {
		normalizedName := strings.ReplaceAll(f.Name, "\\", "/")
		if normalizedName == path || strings.EqualFold(normalizedName, path) {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("file not found: %s", path)
}

// resolvePath resolves a relative path against a base directory
func resolvePath(baseDir, href string) string {
	if baseDir == "" || baseDir == "." {
		return href
	}
	// Handle URL-encoded paths
	href = strings.ReplaceAll(href, "%20", " ")
	return path.Join(baseDir, href)
}

// isHTMLMediaType checks if the media type is HTML/XHTML
func isHTMLMediaType(mediaType string) bool {
	return mediaType == "application/xhtml+xml" ||
		mediaType == "text/html" ||
		mediaType == "application/html"
}

// extractTextFromHTML extracts plain text from HTML content
func extractTextFromHTML(htmlContent []byte) string {
	doc, err := html.Parse(bytes.NewReader(htmlContent))
	if err != nil {
		// Fallback: strip tags with regex
		return stripHTMLTags(string(htmlContent))
	}

	var sb strings.Builder
	extractText(doc, &sb)
	return sb.String()
}

// extractText recursively extracts text from HTML nodes
func extractText(n *html.Node, sb *strings.Builder) {
	// Skip script and style elements
	if n.Type == html.ElementNode {
		switch n.Data {
		case "script", "style", "head", "meta", "link", "title":
			return
		case "p", "div", "br", "h1", "h2", "h3", "h4", "h5", "h6", "li", "tr":
			// Add newline before block elements
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
		}
	}

	if n.Type == html.TextNode {
		text := strings.TrimSpace(n.Data)
		if text != "" {
			if sb.Len() > 0 && !strings.HasSuffix(sb.String(), "\n") && !strings.HasSuffix(sb.String(), " ") {
				sb.WriteString(" ")
			}
			sb.WriteString(text)
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractText(c, sb)
	}
}

// stripHTMLTags is a fallback for when HTML parsing fails
func stripHTMLTags(s string) string {
	// Remove script and style content
	reScript := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	s = reScript.ReplaceAllString(s, "")

	reStyle := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	s = reStyle.ReplaceAllString(s, "")

	// Remove all HTML tags
	reTags := regexp.MustCompile(`<[^>]*>`)
	s = reTags.ReplaceAllString(s, " ")

	// Decode common HTML entities
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")

	return s
}
