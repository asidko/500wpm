package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
)

// CodeGenerator handles unique code generation with collision handling
type CodeGenerator struct {
	defaultLength int
	maxLength     int
}

// NewCodeGenerator creates a new code generator
// defaultLength: starting length for codes (e.g., 6)
// maxLength: maximum length before giving up (e.g., 10)
func NewCodeGenerator(defaultLength, maxLength int) *CodeGenerator {
	if defaultLength < 1 {
		defaultLength = 6
	}
	if maxLength < defaultLength {
		maxLength = defaultLength + 4
	}
	return &CodeGenerator{
		defaultLength: defaultLength,
		maxLength:     maxLength,
	}
}

// CodeExistsFunc is a function that checks if a code already exists
type CodeExistsFunc func(ctx context.Context, code string) (bool, error)

// GenerateUniqueCode generates a unique numeric code
// It starts with defaultLength and increases on collision up to maxLength
func (g *CodeGenerator) GenerateUniqueCode(ctx context.Context, existsFunc CodeExistsFunc) (string, error) {
	currentLength := g.defaultLength

	for currentLength <= g.maxLength {
		// Try up to 10 times at current length before increasing
		for attempt := 0; attempt < 10; attempt++ {
			code, err := g.generateRandomCode(currentLength)
			if err != nil {
				return "", fmt.Errorf("failed to generate random code: %w", err)
			}

			exists, err := existsFunc(ctx, code)
			if err != nil {
				return "", fmt.Errorf("failed to check code existence: %w", err)
			}

			if !exists {
				return code, nil
			}
		}

		// Increase length after 10 failed attempts
		currentLength++
	}

	return "", fmt.Errorf("failed to generate unique code after exhausting all lengths up to %d", g.maxLength)
}

// generateRandomCode generates a random numeric code of specified length
func (g *CodeGenerator) generateRandomCode(length int) (string, error) {
	// Calculate max value (10^length)
	max := big.NewInt(1)
	for i := 0; i < length; i++ {
		max.Mul(max, big.NewInt(10))
	}

	// Generate random number [0, max)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}

	// Format with leading zeros
	format := fmt.Sprintf("%%0%dd", length)
	return fmt.Sprintf(format, n), nil
}

// DefaultLength returns the default code length
func (g *CodeGenerator) DefaultLength() int {
	return g.defaultLength
}

// MaxLength returns the maximum code length
func (g *CodeGenerator) MaxLength() int {
	return g.maxLength
}
