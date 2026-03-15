package index

import (
	"strings"
	"time"
)

// MetadataFilter constrains search results by document metadata.
// Zero-value fields are treated as "no constraint".
type MetadataFilter struct {
	FileTypes      []string  // include only these types (empty = all)
	MinSize        int64     // bytes, 0 = no minimum
	MaxSize        int64     // bytes, 0 = no maximum
	CreatedAfter   time.Time // zero = no constraint
	CreatedBefore  time.Time
	ModifiedAfter  time.Time
	ModifiedBefore time.Time
	Tags           []string  // require ALL listed tags (AND)
}

// IsEmpty returns true if no constraints are set.
func (f *MetadataFilter) IsEmpty() bool {
	return f == nil || (len(f.FileTypes) == 0 &&
		f.MinSize == 0 && f.MaxSize == 0 &&
		f.CreatedAfter.IsZero() && f.CreatedBefore.IsZero() &&
		f.ModifiedAfter.IsZero() && f.ModifiedBefore.IsZero() &&
		len(f.Tags) == 0)
}

// Matches returns true if the document metadata satisfies all filter constraints.
func (f *MetadataFilter) Matches(doc DocMeta) bool {
	if f == nil {
		return true
	}

	if len(f.FileTypes) > 0 {
		found := false
		for _, ft := range f.FileTypes {
			if strings.EqualFold(ft, doc.FileType) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if f.MinSize > 0 && doc.Size < f.MinSize {
		return false
	}
	if f.MaxSize > 0 && doc.Size > f.MaxSize {
		return false
	}

	if !f.CreatedAfter.IsZero() && !doc.CreatedAt.IsZero() && doc.CreatedAt.Before(f.CreatedAfter) {
		return false
	}
	if !f.CreatedBefore.IsZero() && !doc.CreatedAt.IsZero() && doc.CreatedAt.After(f.CreatedBefore) {
		return false
	}

	if !f.ModifiedAfter.IsZero() && !doc.ModifiedAt.IsZero() && doc.ModifiedAt.Before(f.ModifiedAfter) {
		return false
	}
	if !f.ModifiedBefore.IsZero() && !doc.ModifiedAt.IsZero() && doc.ModifiedAt.After(f.ModifiedBefore) {
		return false
	}

	if len(f.Tags) > 0 {
		tagSet := make(map[string]bool)
		for _, t := range doc.Tags {
			tagSet[strings.ToLower(t)] = true
		}
		for _, required := range f.Tags {
			if !tagSet[strings.ToLower(required)] {
				return false
			}
		}
	}

	return true
}

// DetectFileType maps a file extension to a broad file type category.
func DetectFileType(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".txt":
		return "text"
	case ".md", ".markdown":
		return "markdown"
	case ".json":
		return "json"
	case ".xml":
		return "xml"
	case ".html", ".htm":
		return "html"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".cfg", ".conf", ".ini":
		return "config"
	case ".log":
		return "log"
	case ".csv":
		return "data"
	default:
		return "unknown"
	}
}

// DetectMimeType maps a file extension to an approximate MIME type.
func DetectMimeType(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".txt":
		return "text/plain"
	case ".md", ".markdown":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".html", ".htm":
		return "text/html"
	case ".yaml", ".yml":
		return "text/yaml"
	case ".toml":
		return "text/toml"
	case ".csv":
		return "text/csv"
	case ".log", ".cfg", ".conf", ".ini":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

// ReplicaDocInfo holds plaintext metadata shared in encrypted replicas.
// This is intentionally public (not encrypted) — peers need it to filter
// which encrypted vectors to evaluate, reducing computation.
type ReplicaDocInfo struct {
	FileType   string `json:"file_type"`
	Size       int64  `json:"size"`
	CreatedAt  int64  `json:"created_at"`
	ModifiedAt int64  `json:"modified_at"`
	WordCount  int    `json:"word_count"`
}

// DocMetaToReplicaInfo converts a DocMeta to its shareable replica form.
func DocMetaToReplicaInfo(doc DocMeta) ReplicaDocInfo {
	return ReplicaDocInfo{
		FileType:   doc.FileType,
		Size:       doc.Size,
		CreatedAt:  doc.CreatedAt.Unix(),
		ModifiedAt: doc.ModifiedAt.Unix(),
		WordCount:  doc.WordCount,
	}
}

// MatchesReplicaInfo checks if a ReplicaDocInfo satisfies the filter.
func (f *MetadataFilter) MatchesReplicaInfo(info ReplicaDocInfo) bool {
	if f == nil {
		return true
	}

	if len(f.FileTypes) > 0 {
		found := false
		for _, ft := range f.FileTypes {
			if strings.EqualFold(ft, info.FileType) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if f.MinSize > 0 && info.Size < f.MinSize {
		return false
	}
	if f.MaxSize > 0 && info.Size > f.MaxSize {
		return false
	}

	if !f.CreatedAfter.IsZero() && info.CreatedAt < f.CreatedAfter.Unix() {
		return false
	}
	if !f.CreatedBefore.IsZero() && info.CreatedAt > f.CreatedBefore.Unix() {
		return false
	}

	if !f.ModifiedAfter.IsZero() && info.ModifiedAt < f.ModifiedAfter.Unix() {
		return false
	}
	if !f.ModifiedBefore.IsZero() && info.ModifiedAt > f.ModifiedBefore.Unix() {
		return false
	}

	return true
}
