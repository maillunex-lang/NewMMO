// Package content verifies export bundles and manages the content registry.
package content

import (
	"bufio"
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
	"strings"
	"unicode/utf8"
)

const (
	exportFormat   = "newmmo.content.export"
	schemaVersion  = 1
	maxBundleBytes = 64 << 20
	maxLineBytes   = 1 << 20
	maxEntities    = 100_000
	maxJSONDepth   = 16
)

var (
	hashPattern   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	regionPattern = regexp.MustCompile(`^REG-0[1-6]$`)
)

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
	BuildID        string `json:"-"`
	ManifestSHA256 string `json:"-"`
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
	input := bundleInput{dir: dir, remaining: maxBundleBytes}
	var buildID string
	if err := input.read("BUILD_ID", func(r io.Reader) error {
		raw, err := io.ReadAll(io.LimitReader(r, 66))
		if err != nil {
			return err
		}
		if len(raw) != 65 || raw[64] != '\n' || !hashPattern.Match(raw[:64]) {
			return failure(BundleInvalid, "BUILD_ID must contain 64 lowercase hex characters and LF")
		}
		buildID = string(raw[:64])
		return nil
	}); err != nil {
		return bundle{}, err
	}

	var result bundle
	if err := input.read("manifest.json", func(r io.Reader) error {
		raw, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		if digest(raw) != buildID {
			return failure(HashMismatch, "manifest.json SHA-256 differs from BUILD_ID")
		}
		if len(raw) < 2 || raw[len(raw)-1] != '\n' || bytes.ContainsAny(raw[:len(raw)-1], "\r\n") {
			return failure(BundleInvalid, "manifest.json must be one JSON line followed by LF")
		}
		m, err := readManifest(raw)
		if err != nil {
			return err
		}
		m.BuildID, m.ManifestSHA256 = buildID, buildID
		result.manifest, result.manifestJSON = m, raw
		return nil
	}); err != nil {
		return bundle{}, err
	}

	if err := input.read("entities.jsonl", func(r io.Reader) error {
		var err error
		result.entities, err = readEntities(r, result.manifest)
		return err
	}); err != nil {
		return bundle{}, err
	}
	if err := input.read("crossrefs.json", func(r io.Reader) error {
		hash := sha256.New()
		if _, err := io.Copy(hash, r); err != nil {
			return err
		}
		if hex.EncodeToString(hash.Sum(nil)) != result.manifest.CrossrefsSHA256 {
			return failure(HashMismatch, "crossrefs.json SHA-256 mismatch")
		}
		return nil
	}); err != nil {
		return bundle{}, err
	}
	return result, nil
}

type bundleInput struct {
	dir       string
	remaining int64
}

func (in *bundleInput) read(name string, read func(io.Reader) error) error {
	file, err := os.Open(filepath.Join(in.dir, name))
	if err != nil {
		return failure(BundleInvalid, "cannot open "+name)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return failure(BundleInvalid, name+" must be a regular file")
	}
	if info.Size() > in.remaining {
		return failure(BundleTooLarge, "bundle exceeds 64 MiB")
	}
	limit := &io.LimitedReader{R: file, N: in.remaining + 1}
	err = read(limit)
	if limit.N == 0 {
		return failure(BundleTooLarge, "bundle exceeds 64 MiB")
	}
	in.remaining = limit.N - 1
	if err != nil {
		var registryError *RegistryError
		if errors.As(err, &registryError) {
			return err
		}
		return failure(BundleInvalid, "cannot read "+name)
	}
	return nil
}

func readManifest(raw []byte) (manifest, error) {
	fields, err := objectFields(raw)
	if err != nil || !exactKeys(fields, "counts", "crossrefs_sha256", "entities_sha256",
		"entity_count", "format", "lint", "minimum_client_version", "schema_version", "source_files") {
		return manifest{}, failure(BundleInvalid, "manifest.json: invalid object or fields")
	}
	lint, err := objectFields(fields["lint"])
	if err != nil || !exactKeys(lint, "errors", "warnings") || isNull(lint["errors"]) || isNull(lint["warnings"]) {
		return manifest{}, failure(BundleInvalid, "manifest.json: invalid lint summary")
	}
	counts, err := objectFields(fields["counts"])
	if err != nil || len(counts) == 0 {
		return manifest{}, failure(BundleInvalid, "manifest.json: invalid counts")
	}
	var sources []json.RawMessage
	if err := json.Unmarshal(fields["source_files"], &sources); err != nil || len(sources) != 14 {
		return manifest{}, failure(BundleInvalid, "manifest.json: expected 14 source files")
	}
	for _, src := range sources {
		fields, err := objectFields(src)
		if err != nil || !exactKeys(fields, "file", "sha256") {
			return manifest{}, failure(BundleInvalid, "manifest.json: invalid source file fields")
		}
	}

	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return manifest{}, failure(BundleInvalid, "manifest.json: invalid field types")
	}
	if m.Format != exportFormat {
		return manifest{}, failure(BundleInvalid, "manifest.json: unsupported format")
	}
	if m.SchemaVersion != schemaVersion {
		return manifest{}, failure(SchemaUnsupported, "manifest.json: unsupported schema version")
	}
	if m.EntityCount > maxEntities {
		return manifest{}, failure(BundleTooLarge, "manifest.json: more than 100000 entities")
	}
	if m.Lint.Errors != 0 || m.Lint.Warnings < 0 {
		return manifest{}, failure(BundleInvalid, "manifest.json: lint must have zero errors and nonnegative warnings")
	}
	if m.MinimumClientVersion == "" || m.EntityCount <= 0 ||
		!hashPattern.MatchString(m.EntitiesSHA256) || !hashPattern.MatchString(m.CrossrefsSHA256) {
		return manifest{}, failure(BundleInvalid, "manifest.json: invalid client version, entity count or file hashes")
	}
	files := make(map[string]bool, len(m.SourceFiles))
	for _, src := range m.SourceFiles {
		if src.File == "" || files[src.File] || !hashPattern.MatchString(src.SHA256) {
			return manifest{}, failure(BundleInvalid, "manifest.json: invalid or duplicate source file")
		}
		files[src.File] = true
	}
	remaining := m.EntityCount
	for kind, count := range m.Counts {
		if kind == "" || count <= 0 || count > remaining {
			return manifest{}, failure(CountMismatch, "manifest.json: invalid per-kind counts")
		}
		remaining -= count
	}
	if remaining != 0 {
		return manifest{}, failure(CountMismatch, "manifest.json: per-kind counts do not sum to entity_count")
	}
	return m, nil
}

func readEntities(input io.Reader, m manifest) ([]Entity, error) {
	hash := sha256.New()
	reader := bufio.NewReaderSize(io.TeeReader(input, hash), maxLineBytes+1)
	rows := make([]Entity, 0, m.EntityCount)
	ids := make(map[string]bool, m.EntityCount)
	counts := make(map[string]int, len(m.Counts))
	for {
		line, err := reader.ReadSlice('\n')
		if err == io.EOF && len(line) == 0 {
			break
		}
		if err == bufio.ErrBufferFull {
			return nil, failure(BundleTooLarge, "entities.jsonl: line exceeds 1 MiB")
		}
		if err != nil || bytes.IndexByte(line, '\r') >= 0 {
			return nil, failure(BundleInvalid, "entities.jsonl: expected LF line endings and a trailing newline")
		}
		if len(rows) == maxEntities {
			return nil, failure(BundleTooLarge, "entities.jsonl: more than 100000 entities")
		}
		entity, err := readEntity(line[:len(line)-1])
		if err != nil {
			return nil, fmt.Errorf("entities.jsonl:%d: %w", len(rows)+1, err)
		}
		switch {
		case ids[entity.ID]:
			return nil, failure(BundleInvalid, "entities.jsonl: duplicate id")
		case m.Counts[entity.Kind] == 0:
			return nil, failure(CountMismatch, "entities.jsonl: kind missing from manifest counts")
		}
		if len(rows) > 0 {
			prev := rows[len(rows)-1]
			if prev.Kind > entity.Kind || (prev.Kind == entity.Kind && prev.ID > entity.ID) {
				return nil, failure(BundleInvalid, "entities.jsonl: lines are not sorted by kind and id")
			}
		}
		ids[entity.ID] = true
		counts[entity.Kind]++
		rows = append(rows, entity)
	}
	if hex.EncodeToString(hash.Sum(nil)) != m.EntitiesSHA256 {
		return nil, failure(HashMismatch, "entities.jsonl SHA-256 mismatch")
	}
	if len(rows) != m.EntityCount {
		return nil, failure(CountMismatch, "entities.jsonl: line count does not match entity_count")
	}
	if !maps.Equal(counts, m.Counts) {
		return nil, failure(CountMismatch, "entities.jsonl: per-kind counts do not match the manifest")
	}
	return rows, nil
}

func readEntity(line []byte) (Entity, error) {
	fields, err := objectFields(line)
	if err != nil || !exactKeys(fields, "id", "kind", "payload", "region_id", "schema_version", "source_file", "source_line") {
		return Entity{}, failure(BundleInvalid, "expected exactly the seven entity keys in a strict JSON object")
	}
	var version int
	if err := json.Unmarshal(fields["schema_version"], &version); err != nil || version != schemaVersion {
		return Entity{}, failure(SchemaUnsupported, "unsupported entity schema_version")
	}
	var entity Entity
	if err := json.Unmarshal(line, &entity); err != nil || entity.ID == "" || entity.Kind == "" ||
		entity.Source == "" || entity.Line <= 0 || (entity.RegionID != nil && !regionPattern.MatchString(*entity.RegionID)) ||
		len(entity.Payload) == 0 || entity.Payload[0] != '{' ||
		strings.ContainsAny(entity.ID+entity.Kind+entity.Source, "\x1e\x1f") {
		return Entity{}, failure(BundleInvalid, "invalid entity fields")
	}
	return entity, nil
}

func objectFields(raw []byte) (map[string]json.RawMessage, error) {
	if !utf8.Valid(raw) || bytes.HasPrefix(raw, []byte{0xef, 0xbb, 0xbf}) {
		return nil, errors.New("expected UTF-8 without BOM")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, errors.New("expected JSON object")
	}
	if err := checkJSONValue(decoder, start, 1); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("unexpected data after object")
	}
	var fields map[string]json.RawMessage
	err = json.Unmarshal(raw, &fields)
	return fields, err
}

func checkJSONValue(decoder *json.Decoder, token json.Token, depth int) error {
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if depth > maxJSONDepth {
		return errors.New("JSON nesting exceeds 16")
	}
	var keys map[string]struct{}
	if delim == '{' {
		keys = make(map[string]struct{})
	}
	for decoder.More() {
		if keys != nil {
			token, err := decoder.Token()
			key, ok := token.(string)
			if err != nil || !ok {
				return errors.New("invalid object key")
			}
			if _, exists := keys[key]; exists {
				return errors.New("duplicate object key")
			}
			keys[key] = struct{}{}
		}
		value, err := decoder.Token()
		if err != nil {
			return err
		}
		if err := checkJSONValue(decoder, value, depth+1); err != nil {
			return err
		}
	}
	_, err := decoder.Token()
	return err
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
