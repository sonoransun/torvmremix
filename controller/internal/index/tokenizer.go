// Package index provides document indexing with FHE-encrypted search capabilities.
package index

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// commonStopWords are filtered from index terms to reduce noise.
var commonStopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true,
	"at": true, "be": true, "by": true, "for": true, "from": true,
	"has": true, "he": true, "in": true, "is": true, "it": true,
	"its": true, "of": true, "on": true, "or": true, "she": true,
	"that": true, "the": true, "to": true, "was": true, "were": true,
	"will": true, "with": true, "this": true, "but": true, "not": true,
	"you": true, "all": true, "can": true, "had": true, "her": true,
	"one": true, "our": true, "out": true, "do": true, "if": true,
	"me": true, "my": true, "no": true, "so": true, "up": true,
	"we": true,
}

// Tokenize splits text into normalized, deduplicated search terms.
// Applies: Unicode NFD normalization, lowercasing, punctuation removal,
// stop word filtering, and minimum length filtering (3+ chars).
func Tokenize(text string) []string {
	// NFD normalize to decompose accented characters.
	normalized := norm.NFD.String(text)

	// Split on whitespace and punctuation.
	words := strings.FieldsFunc(normalized, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	seen := make(map[string]bool)
	var terms []string
	for _, w := range words {
		w = strings.ToLower(w)

		// Strip non-letter/digit characters remaining after split.
		w = strings.TrimFunc(w, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})

		// Filter short words and stop words.
		if len(w) < 3 {
			continue
		}
		if commonStopWords[w] {
			continue
		}
		if seen[w] {
			continue
		}
		seen[w] = true
		terms = append(terms, w)
	}
	return terms
}
