package content

import (
	"bytes"
	"encoding/json"
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
	var raw bytes.Buffer
	encoder := json.NewEncoder(&raw)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(m.manifestBody); err != nil {
		t.Fatal(err)
	}
	m.BuildID = digest(raw.Bytes())
	dir := t.TempDir()
	writeFile(t, dir, "manifest.json", raw.Bytes())
	writeFile(t, dir, "BUILD_ID", []byte(m.BuildID+"\n"))
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

func TestVerifyHashesExactManifestBytes(t *testing.T) {
	m := fixtureManifest()
	m.MinimumClientVersion = "é<&>\u2028\u2029"
	dir, original := writeFixture(t, m, []byte(fixtureEntities), []byte("{}\n"))
	raw := readFile(t, dir, "manifest.json")
	raw = bytes.ReplaceAll(raw, []byte(`\u2028\u2029`), []byte("\u2028\u2029"))
	id := digest(raw)
	if id == original.BuildID {
		t.Fatal("test did not change the manifest bytes")
	}
	writeFile(t, dir, "manifest.json", raw)
	writeFile(t, dir, "BUILD_ID", []byte(id+"\n"))
	if summary, err := Verify(dir); err != nil || summary.BuildID != id {
		t.Fatalf("raw Unicode manifest returned %+v, %v", summary, err)
	}
}

func TestVerifyTamperedHashes(t *testing.T) {
	for _, tc := range []struct {
		file, old, new, reason string
	}{
		{"manifest.json", "0.0.1", "0.0.2", "SHA-256 differs"},
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
		{"manifest schema", func(m *manifest, s string) string { m.SchemaVersion++; return s }, "unsupported schema"},
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
	if err == nil || !strings.Contains(err.Error(), reason) {
		t.Fatalf("Verify error = %v, want %q", err, reason)
	}
}

func TestStrictParsingAndLimits(t *testing.T) {
	for _, tc := range []struct {
		name, from, to string
		code           Code
	}{
		{"UTF-8", "Страж", "\xff", BundleInvalid},
		{"BOM", `{"id"`, "\xef\xbb\xbf{\"id\"", BundleInvalid},
		{"duplicate nested key", `"name":"Страж <&>"`, `"name":1,"name":2`, BundleInvalid},
		{"NaN", `"Страж <&>"`, "NaN", BundleInvalid},
		{"Infinity", `"Страж <&>"`, "Infinity", BundleInvalid},
		{"negative infinity", `"Страж <&>"`, "-Infinity", BundleInvalid},
		{"array instead of payload", `{"name":"Страж <&>"}`, "[]", BundleInvalid},
		{"array instead of line", strings.Split(fixtureEntities, "\n")[0], "[]", BundleInvalid},
		{"CRLF", "\n", "\r\n", BundleInvalid},
		{"depth", `"Страж <&>"`, strings.Repeat("[", 15) + "0" + strings.Repeat("]", 15), BundleInvalid},
		{"oversize line", "Страж", strings.Repeat("x", maxLineBytes), BundleTooLarge},
		{"missing entity", strings.Split(fixtureEntities, "\n")[0] + "\n", "", CountMismatch},
		{"extra key", `"region_id":null`, `"region_id":null,"extra":1`, BundleInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := strings.Replace(fixtureEntities, tc.from, tc.to, 1)
			dir, _ := writeFixture(t, fixtureManifest(), []byte(data), []byte("{}\n"))
			if _, err := Verify(dir); err == nil || ErrorCode(err) != tc.code {
				t.Fatalf("Verify error = %v; want %s", err, tc.code)
			}
		})
	}
	t.Run("bundle too large", func(t *testing.T) {
		dir, _ := writeFixture(t, fixtureManifest(), []byte(fixtureEntities), []byte("{}\n"))
		if err := os.Truncate(filepath.Join(dir, "crossrefs.json"), maxBundleBytes); err != nil {
			t.Fatal(err)
		}
		if _, err := Verify(dir); err == nil || ErrorCode(err) != BundleTooLarge {
			t.Fatalf("Verify error = %v; want %s", err, BundleTooLarge)
		}
	})
	t.Run("entity count limit", func(t *testing.T) {
		m := fixtureManifest()
		m.EntityCount, m.Counts["class"] = maxEntities+1, maxEntities
		dir, _ := writeFixture(t, m, []byte(fixtureEntities), []byte("{}\n"))
		if _, err := Verify(dir); err == nil || ErrorCode(err) != BundleTooLarge {
			t.Fatalf("Verify error = %v; want %s", err, BundleTooLarge)
		}
	})
	t.Run("manifest one line and LF", func(t *testing.T) {
		dir, _ := writeFixture(t, fixtureManifest(), []byte(fixtureEntities), []byte("{}\n"))
		raw := append(readFile(t, dir, "manifest.json"), '\n')
		writeFile(t, dir, "manifest.json", raw)
		writeFile(t, dir, "BUILD_ID", []byte(digest(raw)+"\n"))
		if _, err := Verify(dir); err == nil || ErrorCode(err) != BundleInvalid {
			t.Fatalf("Verify error = %v; want %s", err, BundleInvalid)
		}
	})
}
