// Package content verifies export bundles and manages the content registry.
package content

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"unicode/utf8"
)

const (
	exportFormat  = "newmmo.content.export"
	schemaVersion = 1
)

var (
	ErrInvalidBundle = errors.New("invalid content bundle")
	hashPattern      = regexp.MustCompile(`^[0-9a-f]{64}$`)
	regionPattern    = regexp.MustCompile(`^REG-[0-9]{2}$`)
)

// Manifest body fields (including nested objects) are ordered by JSON key.
type manifestBody struct {
	Counts               map[string]int `json:"counts"`
	CrossrefsSHA256      string         `json:"crossrefs_sha256"`
	EntitiesSHA256       string         `json:"entities_sha256"`
	EntityCount          int            `json:"entity_count"`
	Format               string         `json:"format"`
	Lint                 lintSummary    `json:"lint"`
	MinimumClientVersion string         `json:"minimum_client_version"`
	SchemaVersion        int            `json:"schema_version"`
	SourceFiles          []sourceFile   `json:"source_files"`
}

type lintSummary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

type sourceFile struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

type manifest struct {
	manifestBody
	BuildID        string `json:"build_id"`
	ManifestSHA256 string `json:"manifest_sha256"`
}

type Entity struct {
	ID       string          `json:"id"`
	Kind     string          `json:"kind"`
	Payload  json.RawMessage `json:"payload"`
	RegionID *string         `json:"region_id"`
	Source   string          `json:"source_file"`
	Line     int             `json:"source_line"`
}

type Summary struct {
	BuildID     string `json:"build_id"`
	EntityCount int    `json:"entity_count"`
}

type bundle struct {
	manifest     manifest
	manifestJSON []byte
	entities     []Entity
}

func (b bundle) summary() Summary {
	return Summary{BuildID: b.manifest.BuildID, EntityCount: b.manifest.EntityCount}
}

// Verify validates a bundle without opening a database connection.
func Verify(dir string) (Summary, error) {
	b, err := loadBundle(dir)
	if err != nil {
		return Summary{}, err
	}
	return b.summary(), nil
}

func loadBundle(dir string) (bundle, error) {
	b, err := readBundle(dir)
	if err != nil {
		return bundle{}, fmt.Errorf("%w: %v", ErrInvalidBundle, err)
	}
	return b, nil
}

func readBundle(dir string) (bundle, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return bundle{}, fmt.Errorf("read manifest.json: %w", err)
	}
	m, err := readManifest(raw)
	if err != nil {
		return bundle{}, err
	}
	entities, err := readHashedFile(dir, "entities.jsonl", m.EntitiesSHA256)
	if err != nil {
		return bundle{}, err
	}
	if _, err := readHashedFile(dir, "crossrefs.json", m.CrossrefsSHA256); err != nil {
		return bundle{}, err
	}
	rows, err := readEntities(entities, m)
	if err != nil {
		return bundle{}, err
	}
	return bundle{manifest: m, manifestJSON: raw, entities: rows}, nil
}

func readManifest(raw []byte) (manifest, error) {
	fields, err := objectFields(raw)
	if err != nil || !exactKeys(fields, "build_id", "counts", "crossrefs_sha256", "entities_sha256",
		"entity_count", "format", "lint", "manifest_sha256", "minimum_client_version", "schema_version", "source_files") {
		return manifest{}, errors.New("manifest.json: invalid object or fields")
	}
	lint, err := objectFields(fields["lint"])
	if err != nil || !exactKeys(lint, "errors", "warnings") || isNull(lint["errors"]) || isNull(lint["warnings"]) {
		return manifest{}, errors.New("manifest.json: invalid lint summary")
	}
	counts, err := objectFields(fields["counts"])
	if err != nil || len(counts) == 0 {
		return manifest{}, errors.New("manifest.json: invalid counts")
	}
	var sources []json.RawMessage
	if err := json.Unmarshal(fields["source_files"], &sources); err != nil || len(sources) != 14 {
		return manifest{}, errors.New("manifest.json: expected 14 source files")
	}
	for _, src := range sources {
		fields, err := objectFields(src)
		if err != nil || !exactKeys(fields, "file", "sha256") {
			return manifest{}, errors.New("manifest.json: invalid source file fields")
		}
	}

	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return manifest{}, errors.New("manifest.json: invalid field types")
	}
	if m.Format != exportFormat || m.SchemaVersion != schemaVersion {
		return manifest{}, errors.New("manifest.json: unsupported format or schema version")
	}
	if m.Lint.Errors != 0 || m.Lint.Warnings < 0 {
		return manifest{}, errors.New("manifest.json: lint must have zero errors and nonnegative warnings")
	}
	if m.MinimumClientVersion == "" || m.EntityCount <= 0 ||
		!hashPattern.MatchString(m.EntitiesSHA256) || !hashPattern.MatchString(m.CrossrefsSHA256) {
		return manifest{}, errors.New("manifest.json: invalid client version, entity count or file hashes")
	}
	files := make(map[string]bool, len(m.SourceFiles))
	for _, src := range m.SourceFiles {
		if src.File == "" || files[src.File] || !hashPattern.MatchString(src.SHA256) {
			return manifest{}, errors.New("manifest.json: invalid or duplicate source file")
		}
		files[src.File] = true
	}
	remaining := m.EntityCount
	for kind, count := range m.Counts {
		if kind == "" || count <= 0 || count > remaining {
			return manifest{}, errors.New("manifest.json: invalid per-kind counts")
		}
		remaining -= count
	}
	if remaining != 0 {
		return manifest{}, errors.New("manifest.json: per-kind counts do not sum to entity_count")
	}
	canonical, err := canonicalManifest(m.manifestBody)
	if err != nil {
		return manifest{}, fmt.Errorf("encode canonical manifest: %w", err)
	}
	if m.ManifestSHA256 != digest(canonical) || m.BuildID != m.ManifestSHA256 {
		return manifest{}, errors.New("manifest.json: manifest_sha256 or build_id mismatch")
	}
	return m, nil
}

func canonicalManifest(body manifestBody) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(body); err != nil {
		return nil, err
	}

	// encoding/json always escapes these separators. The exporter writes
	// them as UTF-8. Skip escaped backslashes so literal text stays intact.
	raw := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	canonical := make([]byte, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\\' {
			switch {
			case bytes.HasPrefix(raw[i:], []byte(`\u2028`)):
				canonical = append(canonical, "\u2028"...)
				i += 5
				continue
			case bytes.HasPrefix(raw[i:], []byte(`\u2029`)):
				canonical = append(canonical, "\u2029"...)
				i += 5
				continue
			default:
				canonical = append(canonical, raw[i], raw[i+1])
				i++
				continue
			}
		}
		canonical = append(canonical, raw[i])
	}
	return canonical, nil
}

func readHashedFile(dir, name, want string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	if digest(data) != want {
		return nil, fmt.Errorf("%s: SHA-256 mismatch", name)
	}
	return data, nil
}

func readEntities(raw []byte, m manifest) ([]Entity, error) {
	if !bytes.HasSuffix(raw, []byte("\n")) || bytes.Contains(raw, []byte("\r")) {
		return nil, errors.New("entities.jsonl: expected LF line endings and a trailing newline")
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte("\n"))
	if len(lines) != m.EntityCount {
		return nil, errors.New("entities.jsonl: line count does not match entity_count")
	}

	rows := make([]Entity, 0, len(lines))
	ids := make(map[string]bool, len(lines))
	counts := make(map[string]int, len(m.Counts))
	for i, line := range lines {
		entity, err := readEntity(line)
		if err != nil {
			return nil, fmt.Errorf("entities.jsonl:%d: %w", i+1, err)
		}
		if ids[entity.ID] {
			return nil, fmt.Errorf("entities.jsonl:%d: duplicate id", i+1)
		}
		if _, ok := m.Counts[entity.Kind]; !ok {
			return nil, fmt.Errorf("entities.jsonl:%d: kind missing from manifest counts", i+1)
		}
		if i > 0 {
			prev := rows[i-1]
			if prev.Kind > entity.Kind || (prev.Kind == entity.Kind && prev.ID > entity.ID) {
				return nil, fmt.Errorf("entities.jsonl:%d: lines are not sorted by kind and id", i+1)
			}
		}
		ids[entity.ID] = true
		counts[entity.Kind]++
		rows = append(rows, entity)
	}
	if !maps.Equal(counts, m.Counts) {
		return nil, errors.New("entities.jsonl: per-kind counts do not match the manifest")
	}
	return rows, nil
}

func readEntity(line []byte) (Entity, error) {
	fields, err := objectFields(line)
	if err != nil || !exactKeys(fields, "id", "kind", "payload", "region_id", "schema_version", "source_file", "source_line") {
		return Entity{}, errors.New("expected exactly the seven entity keys")
	}
	var version int
	if err := json.Unmarshal(fields["schema_version"], &version); err != nil || version != schemaVersion {
		return Entity{}, errors.New("unsupported entity schema_version")
	}
	var entity Entity
	if err := json.Unmarshal(line, &entity); err != nil || entity.ID == "" || entity.Kind == "" ||
		entity.Source == "" || entity.Line <= 0 || (entity.RegionID != nil && !regionPattern.MatchString(*entity.RegionID)) ||
		len(entity.Payload) == 0 || entity.Payload[0] != '{' {
		return Entity{}, errors.New("invalid entity fields")
	}
	return entity, nil
}

func objectFields(raw []byte) (map[string]json.RawMessage, error) {
	if !utf8.Valid(raw) {
		return nil, errors.New("invalid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, errors.New("expected JSON object")
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok {
			return nil, errors.New("invalid object key")
		}
		if _, exists := fields[key]; exists {
			return nil, errors.New("duplicate object key")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		fields[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("unexpected data after object")
	}
	return fields, nil
}

func exactKeys(fields map[string]json.RawMessage, keys ...string) bool {
	if len(fields) != len(keys) {
		return false
	}
	for _, key := range keys {
		if _, ok := fields[key]; !ok {
			return false
		}
	}
	return true
}

func isNull(raw json.RawMessage) bool { return bytes.Equal(raw, []byte("null")) }

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
