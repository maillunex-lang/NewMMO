package content

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureEntities = `{"id":"CLS-01","kind":"class","payload":{"name":"Страж <&>"},"region_id":null,"schema_version":1,"source_file":"01_TEST.md","source_line":1}
{"id":"LOC-01-001","kind":"location","payload":{"name":"Долина"},"region_id":"REG-01","schema_version":1,"source_file":"01_TEST.md","source_line":2}
`

func fixtureManifest() manifest {
	sourceFiles := make([]sourceFile, 14)
	for i := range sourceFiles {
		name := fmt.Sprintf("%02d_TEST.md", i+1)
		sourceFiles[i] = sourceFile{File: name, SHA256: digest([]byte(name))}
	}
	return manifest{manifestBody: manifestBody{
		Counts: map[string]int{"class": 1, "location": 1}, EntityCount: 2, Format: exportFormat,
		MinimumClientVersion: "0.0.1", SchemaVersion: schemaVersion, SourceFiles: sourceFiles,
	}}
}

func writeFixture(t *testing.T, m manifest, entities, crossrefs []byte) (string, Summary) {
	t.Helper()
	m.EntitiesSHA256 = digest(entities)
	m.CrossrefsSHA256 = digest(crossrefs)
	body, err := canonicalManifest(m.manifestBody)
	if err != nil {
		t.Fatal(err)
	}
	m.ManifestSHA256 = digest(body)
	m.BuildID = m.ManifestSHA256
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeFile(t, dir, "manifest.json", raw)
	writeFile(t, dir, "entities.jsonl", entities)
	writeFile(t, dir, "crossrefs.json", crossrefs)
	return dir, Summary{BuildID: m.BuildID, EntityCount: m.EntityCount}
}

func writeFile(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, dir, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestVerifyBundle(t *testing.T) {
	dir, want := writeFixture(t, fixtureManifest(), []byte(fixtureEntities), []byte("{}\n"))
	summary, err := Verify(dir)
	if err != nil || summary != want {
		t.Fatalf("Verify = %+v, %v; want %+v", summary, err, want)
	}
	b, err := loadBundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.entities[0].RegionID != nil || string(b.entities[0].Payload) != `{"name":"Страж <&>"}` ||
		b.entities[1].RegionID == nil || *b.entities[1].RegionID != "REG-01" {
		t.Fatalf("unexpected decoded entities: %+v", b.entities)
	}
}

func TestCanonicalManifestUnicode(t *testing.T) {
	body := fixtureManifest().manifestBody
	body.MinimumClientVersion = "é<&>\u2028\u2029" + `\u2028`
	raw, err := canonicalManifest(body)
	if err != nil {
		t.Fatal(err)
	}
	want := `"minimum_client_version":"é<&>` + "\u2028\u2029" + `\\u2028"`
	if !bytes.Contains(raw, []byte(want)) || !bytes.HasPrefix(raw, []byte(`{"counts":{"class":1,"location":1},`)) {
		t.Fatalf("unexpected canonical bytes: %q", raw)
	}
}

func TestVerifyTamperedHashes(t *testing.T) {
	for _, tc := range []struct {
		file, old, new, reason string
	}{
		{"manifest.json", "0.0.1", "0.0.2", "manifest_sha256 or build_id mismatch"},
		{"entities.jsonl", "Страж", "Подмена", "SHA-256 mismatch"},
		{"crossrefs.json", "{}", "{ }", "SHA-256 mismatch"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			dir, _ := writeFixture(t, fixtureManifest(), []byte(fixtureEntities), []byte("{}\n"))
			data := readFile(t, dir, tc.file)
			writeFile(t, dir, tc.file, bytes.Replace(data, []byte(tc.old), []byte(tc.new), 1))
			assertInvalidBundle(t, dir, tc.reason)
		})
	}
}

func TestVerifyRejectsInvalidManifestAndEntities(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*manifest, string) string
		reason string
	}{
		{"manifest schema", func(m *manifest, s string) string { m.SchemaVersion++; return s }, "unsupported format or schema"},
		{"lint errors", func(m *manifest, s string) string { m.Lint.Errors++; return s }, "lint must have zero errors"},
		{"wrong count sum", func(m *manifest, s string) string { m.EntityCount++; return s }, "do not sum"},
		{"wrong line count", func(m *manifest, s string) string { m.EntityCount++; m.Counts["class"]++; return s }, "line count"},
		{"wrong kind counts", func(_ *manifest, s string) string {
			return strings.ReplaceAll(s, `"kind":"location"`, `"kind":"class"`)
		}, "per-kind counts do not match"},
		{"unknown kind", func(_ *manifest, s string) string {
			return strings.ReplaceAll(s, `"kind":"location"`, `"kind":"unknown"`)
		}, "kind missing"},
		{"duplicate id across kinds", func(_ *manifest, s string) string {
			return strings.ReplaceAll(s, "LOC-01-001", "CLS-01")
		}, "duplicate id"},
		{"missing key", func(_ *manifest, s string) string {
			return strings.Replace(s, `"region_id":null,`, "", 1)
		}, "seven entity keys"},
		{"duplicate key", func(_ *manifest, s string) string {
			return strings.Replace(s, `"region_id":null,`, `"region_id":null,"region_id":null,`, 1)
		}, "seven entity keys"},
		{"entity schema", func(_ *manifest, s string) string {
			return strings.Replace(s, `"schema_version":1`, `"schema_version":2`, 1)
		}, "unsupported entity schema"},
		{"unsorted", func(_ *manifest, s string) string {
			first, rest, _ := strings.Cut(s, "\n")
			return rest + first + "\n"
		}, "not sorted"},
		{"missing trailing LF", func(_ *manifest, s string) string { return strings.TrimSuffix(s, "\n") }, "trailing newline"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fixtureManifest()
			entities := tc.change(&m, fixtureEntities)
			dir, _ := writeFixture(t, m, []byte(entities), []byte("{}\n"))
			assertInvalidBundle(t, dir, tc.reason)
		})
	}
}

func assertInvalidBundle(t *testing.T, dir, reason string) {
	t.Helper()
	_, err := Verify(dir)
	if !errors.Is(err, ErrInvalidBundle) || !strings.Contains(err.Error(), reason) {
		t.Fatalf("Verify error = %v, want %q", err, reason)
	}
}
