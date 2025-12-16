package converter

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/500wpm/backend/internal/models"
)

// MOBIConverter handles MOBI/AZW/PRC files
type MOBIConverter struct{}

// SupportedFormat returns the format this converter handles
func (c *MOBIConverter) SupportedFormat() string {
	return FormatMOBI
}

// PDB header structure
type pdbHeader struct {
	Name       [32]byte
	Attributes uint16
	Version    uint16
	CTime      uint32
	MTime      uint32
	BTime      uint32
	ModNum     uint32
	AppInfo    uint32
	SortInfo   uint32
	Type       [4]byte
	Creator    [4]byte
	UniqueID   uint32
	NextRec    uint32
	NumRecords uint16
}

// Record info structure
type recordInfo struct {
	Offset     uint32
	Attributes uint8
	UniqueID   [3]byte
}

// PalmDOC header
type palmDocHeader struct {
	Compression uint16
	Unused      uint16
	TextLength  uint32
	RecordCount uint16
	RecordSize  uint16
	CurrentPos  uint32
}

// Compression types
const (
	compressionNone    = 1
	compressionPalmDOC = 2
	compressionHUFF    = 17480
)

// Convert parses a MOBI file into BookContent
func (c *MOBIConverter) Convert(content []byte, filename string) (*models.BookContent, error) {
	if len(content) < 78 {
		return nil, fmt.Errorf("%w: file too small", ErrInvalidFile)
	}

	reader := bytes.NewReader(content)

	// Read PDB header
	var pdb pdbHeader
	if err := binary.Read(reader, binary.BigEndian, &pdb); err != nil {
		return nil, fmt.Errorf("%w: failed to read PDB header", ErrInvalidFile)
	}

	// Verify file type
	typeStr := string(pdb.Type[:])
	creatorStr := string(pdb.Creator[:])
	if typeStr != "BOOK" && typeStr != "TEXt" {
		return nil, fmt.Errorf("%w: not a MOBI file (type: %s)", ErrInvalidFile, typeStr)
	}
	if creatorStr != "MOBI" && creatorStr != "REAd" {
		return nil, fmt.Errorf("%w: not a MOBI file (creator: %s)", ErrInvalidFile, creatorStr)
	}

	// Read record info list
	records := make([]recordInfo, pdb.NumRecords)
	for i := uint16(0); i < pdb.NumRecords; i++ {
		if err := binary.Read(reader, binary.BigEndian, &records[i]); err != nil {
			return nil, fmt.Errorf("%w: failed to read record info", ErrInvalidFile)
		}
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("%w: not enough records", ErrInvalidFile)
	}

	// Get first record for headers
	firstRecordEnd := uint32(len(content))
	if len(records) > 1 {
		firstRecordEnd = records[1].Offset
	}
	if records[0].Offset >= firstRecordEnd || int(firstRecordEnd) > len(content) {
		return nil, fmt.Errorf("%w: invalid record offset", ErrInvalidFile)
	}

	firstRecord := content[records[0].Offset:firstRecordEnd]
	if len(firstRecord) < 16 {
		return nil, fmt.Errorf("%w: first record too small", ErrInvalidFile)
	}

	// Parse PalmDOC header
	var palmDoc palmDocHeader
	palmDocReader := bytes.NewReader(firstRecord)
	if err := binary.Read(palmDocReader, binary.BigEndian, &palmDoc); err != nil {
		return nil, fmt.Errorf("%w: failed to read PalmDOC header", ErrInvalidFile)
	}

	// Check compression type
	if palmDoc.Compression == compressionHUFF {
		return nil, fmt.Errorf("%w: HUFF/CDIC compression not supported", ErrInvalidFile)
	}

	// Try to extract metadata from MOBI header
	var title string
	var author string
	var encoding uint32 = 65001 // Default UTF-8

	if len(firstRecord) >= 132 {
		mobiMagic := string(firstRecord[16:20])
		if mobiMagic == "MOBI" {
			// Extract encoding
			encoding = binary.BigEndian.Uint32(firstRecord[28:32])

			// Try to get full title from MOBI header
			if len(firstRecord) >= 92 {
				fullNameOffset := binary.BigEndian.Uint32(firstRecord[84:88])
				fullNameLength := binary.BigEndian.Uint32(firstRecord[88:92])

				if fullNameOffset > 0 && fullNameLength > 0 {
					nameStart := records[0].Offset + fullNameOffset
					nameEnd := nameStart + fullNameLength
					if int(nameEnd) <= len(content) && nameEnd > nameStart {
						title = strings.TrimRight(string(content[nameStart:nameEnd]), "\x00")
					}
				}
			}

			// Try to parse EXTH for author
			if len(firstRecord) >= 128 {
				exthFlags := binary.BigEndian.Uint32(firstRecord[128:132])
				if exthFlags&0x40 != 0 {
					// EXTH header exists
					mobiHeaderLen := binary.BigEndian.Uint32(firstRecord[20:24])
					exthOffset := 16 + mobiHeaderLen

					if int(exthOffset)+12 <= len(firstRecord) {
						exthMagic := string(firstRecord[exthOffset : exthOffset+4])
						if exthMagic == "EXTH" {
							author = c.parseEXTH(firstRecord[exthOffset:], &title)
						}
					}
				}
			}
		}
	}

	// Fall back to PDB name
	if title == "" {
		title = strings.TrimRight(string(pdb.Name[:]), "\x00")
	}
	if title == "" {
		title = extractTitle(filename)
	}

	// Calculate number of text records
	textRecordCount := int(palmDoc.RecordCount)
	if textRecordCount <= 0 || textRecordCount >= len(records) {
		textRecordCount = len(records) - 1
	}
	if textRecordCount > 1000 {
		textRecordCount = 1000
	}

	// Extract and decompress text records
	var textBuilder strings.Builder
	var totalBytes uint32 = 0

	for i := 1; i <= textRecordCount && i < len(records); i++ {
		recordStart := records[i].Offset
		recordEnd := uint32(len(content))
		if i+1 < len(records) {
			recordEnd = records[i+1].Offset
		}

		if int(recordEnd) > len(content) || recordStart >= recordEnd {
			continue
		}

		recordData := content[recordStart:recordEnd]

		var text []byte
		switch palmDoc.Compression {
		case compressionNone:
			text = recordData
		case compressionPalmDOC:
			text = decompressPalmDOC(recordData)
		default:
			text = recordData
		}

		textBuilder.Write(text)
		totalBytes += uint32(len(text))

		// Stop if we've read expected text length
		if palmDoc.TextLength > 0 && totalBytes >= palmDoc.TextLength {
			break
		}
	}

	rawText := textBuilder.String()

	// Handle text encoding
	text := c.decodeText(rawText, encoding)

	// Clean up HTML
	text = cleanMobiText(text)

	if len(strings.TrimSpace(text)) < 100 {
		return nil, fmt.Errorf("%w: no readable content found", ErrInvalidFile)
	}

	chapters := []models.Chapter{
		{
			Title:   title,
			Content: text,
		},
	}

	return models.NewBookContent(title, author, "", FormatMOBI, chapters), nil
}

// parseEXTH extracts author and potentially updates title from EXTH header
func (c *MOBIConverter) parseEXTH(exthData []byte, title *string) string {
	if len(exthData) < 12 {
		return ""
	}

	// EXTH header: magic(4) + length(4) + count(4)
	recordCount := binary.BigEndian.Uint32(exthData[8:12])
	if recordCount > 100 {
		recordCount = 100 // Sanity limit
	}

	var author string
	offset := uint32(12)

	for i := uint32(0); i < recordCount && int(offset)+8 <= len(exthData); i++ {
		recordType := binary.BigEndian.Uint32(exthData[offset : offset+4])
		recordLen := binary.BigEndian.Uint32(exthData[offset+4 : offset+8])

		if recordLen < 8 || int(offset+recordLen) > len(exthData) {
			break
		}

		valueLen := recordLen - 8
		if valueLen > 0 {
			value := string(exthData[offset+8 : offset+8+valueLen])
			value = strings.TrimRight(value, "\x00")

			switch recordType {
			case 100: // Author
				author = value
			case 503: // Updated title
				if *title == "" || len(value) > 0 {
					*title = value
				}
			}
		}

		offset += recordLen
	}

	return author
}

// decodeText converts text to UTF-8 based on encoding
func (c *MOBIConverter) decodeText(text string, encoding uint32) string {
	if encoding == 65001 || utf8.ValidString(text) {
		return text
	}

	// CP1252 to Unicode mapping for bytes 0x80-0x9F
	cp1252Map := map[byte]rune{
		0x80: 0x20AC, 0x82: 0x201A, 0x83: 0x0192, 0x84: 0x201E,
		0x85: 0x2026, 0x86: 0x2020, 0x87: 0x2021, 0x88: 0x02C6,
		0x89: 0x2030, 0x8A: 0x0160, 0x8B: 0x2039, 0x8C: 0x0152,
		0x8E: 0x017D, 0x91: 0x2018, 0x92: 0x2019, 0x93: 0x201C,
		0x94: 0x201D, 0x95: 0x2022, 0x96: 0x2013, 0x97: 0x2014,
		0x98: 0x02DC, 0x99: 0x2122, 0x9A: 0x0161, 0x9B: 0x203A,
		0x9C: 0x0153, 0x9E: 0x017E, 0x9F: 0x0178,
	}

	result := make([]rune, 0, len(text))
	for i := 0; i < len(text); i++ {
		b := text[i]
		if b < 128 {
			result = append(result, rune(b))
		} else if r, ok := cp1252Map[b]; ok {
			result = append(result, r)
		} else {
			result = append(result, rune(b))
		}
	}
	return string(result)
}

// cleanMobiText strips HTML and cleans up MOBI text
func cleanMobiText(text string) string {
	// Remove script and style content
	reScript := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	text = reScript.ReplaceAllString(text, "")

	reStyle := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	text = reStyle.ReplaceAllString(text, "")

	// Replace block elements with newlines
	reBlock := regexp.MustCompile(`(?i)</(p|div|h[1-6]|li|tr)[^>]*>`)
	text = reBlock.ReplaceAllString(text, "\n")

	reBr := regexp.MustCompile(`(?i)<br[^>]*/?>\s*`)
	text = reBr.ReplaceAllString(text, "\n")

	// Remove remaining tags
	reTags := regexp.MustCompile(`<[^>]*>`)
	text = reTags.ReplaceAllString(text, "")

	// Decode HTML entities
	text = html.UnescapeString(text)

	// Replace non-breaking spaces
	text = strings.ReplaceAll(text, "\u00A0", " ")

	// Normalize whitespace
	text = models.NormalizeText(text)

	return text
}

// decompressPalmDOC decompresses PalmDOC compressed data
func decompressPalmDOC(data []byte) []byte {
	var result []byte
	i := 0

	for i < len(data) {
		b := data[i]
		i++

		if b == 0 {
			result = append(result, 0)
		} else if b >= 1 && b <= 8 {
			count := int(b)
			for j := 0; j < count && i < len(data); j++ {
				result = append(result, data[i])
				i++
			}
		} else if b >= 9 && b <= 0x7F {
			result = append(result, b)
		} else if b >= 0x80 && b <= 0xBF {
			if i >= len(data) {
				break
			}
			next := data[i]
			i++

			distance := ((int(b) & 0x3F) << 8) | int(next)
			distance >>= 3
			length := (int(next) & 0x07) + 3

			if distance > 0 && distance <= len(result) {
				start := len(result) - distance
				for j := 0; j < length; j++ {
					if start+j < len(result) {
						result = append(result, result[start+j])
					}
				}
			}
		} else if b >= 0xC0 {
			result = append(result, ' ')
			result = append(result, b^0x80)
		}
	}

	return result
}
