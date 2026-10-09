package registry

import (
	"encoding/json"
	"github.com/formancehq/fctl/pkg/pluginsdk"
	"strings"
	"testing"
)

func hash(s string) string { return strings.Repeat(s, 64) }
func reference() Reference {
	return Reference{ServiceVersion: "2.5.2", Catalogue: "https://example.org/catalogue.json", SHA256: hash("a")}
}
func release() Release {
	return Release{Service: "auth", ServiceVersion: "2.5.2", Revision: 1, Platform: Platform{"linux", "amd64"}, Artifact: Artifact{"https://ghcr.io", "formancehq/fctl-plugin-auth", "sha256:" + hash("b")}, SHA256: hash("c"), Manifest: pluginsdk.Manifest{Name: "auth", Service: "auth", Version: "2.5.2", ProtocolVersion: 1, Root: pluginsdk.CommandSpec{Use: "auth", Args: pluginsdk.ArgsSpec{Min: 0, Max: 0}, Subcommands: []pluginsdk.CommandSpec{{Use: "info", Runnable: true}}}}}
}
func encoded(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestIndex(t *testing.T) {
	valid := `schemaVersion: 2
plugins:
  auth:
    releases:
      - serviceVersion: 2.5.2
        catalogue: https://example.org/catalogue.json
        sha256: ` + hash("a") + `\n`
	valid = strings.ReplaceAll(valid, `\n`, "\n")
	cases := map[string]string{"valid": valid, "unknown": valid + "manifests: []\n", "duplicate": valid + "schemaVersion: 2\n", "duplicate service": valid + "  auth: {releases: []}\n", "wrong schema": strings.Replace(valid, "schemaVersion: 2", "schemaVersion: 1", 1), "embedded": strings.Replace(valid, "catalogue:", "manifest:", 1), "bad hash": strings.Replace(valid, hash("a"), hash("A"), 1), "insecure": strings.Replace(valid, "https:", "http:", 1), "prefixed version": strings.Replace(valid, "2.5.2", "v2.5.2", 1), "bad version": strings.Replace(valid, "2.5.2", "latest", 1), "missing plugins": "schemaVersion: 2", "null releases": "schemaVersion: 2\nplugins: {auth: {releases: null}}", "invalid service": strings.Replace(valid, "  auth:", "  ../auth:", 1), "extra document": valid + "---\nschemaVersion: 2", "alias": "schemaVersion: 2\nplugins: &a {auth: *a}"}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseIndex([]byte(data))
			if (err == nil) != (name == "valid") {
				t.Fatalf("got %v", err)
			}
		})
	}
	index := Index{2, map[string]Product{"auth": {[]Reference{reference(), reference()}}}}
	if _, err := ParseIndex(encoded(t, index)); err == nil {
		t.Fatal("duplicate version accepted")
	}
}
func TestCatalogueRejectsDrift(t *testing.T) {
	changes := map[string]func(*Release){"valid": func(*Release) {}, "service": func(r *Release) { r.Service = "ledger" }, "version": func(r *Release) { r.ServiceVersion = "2.5.3" }, "revision": func(r *Release) { r.Revision = 0 }, "os": func(r *Release) { r.Platform.OS = "plan9" }, "arch": func(r *Release) { r.Platform.Arch = "386" }, "binary hash": func(r *Release) { r.SHA256 = hash("A") }, "manifest digest": func(r *Release) { r.Artifact.Digest = hash("a") }, "registry": func(r *Release) { r.Artifact.Registry = "http://ghcr.io" }, "registry path": func(r *Release) { r.Artifact.Registry = "https://ghcr.io/v2" }, "repository": func(r *Release) { r.Artifact.Repository = "../bad" }, "protocol": func(r *Release) { r.Manifest.ProtocolVersion = 2 }, "manifest service": func(r *Release) { r.Manifest.Service = "ledger" }, "manifest version": func(r *Release) { r.Manifest.Version = "2.5.3" }, "root": func(r *Release) { r.Manifest.Root.Use = "ledger" }, "command args": func(r *Release) { r.Manifest.Root.Args.Min = -1 }, "duplicate command": func(r *Release) {
		r.Manifest.Root.Subcommands = append(r.Manifest.Root.Subcommands, r.Manifest.Root.Subcommands[0])
	}, "invalid child": func(r *Release) { r.Manifest.Root.Subcommands[0].Use = "../bad" }, "flag type": func(r *Release) { r.Manifest.Root.Flags = []pluginsdk.FlagSpec{{Name: "bad", Type: "mystery"}} }, "flag duplicate": func(r *Release) {
		r.Manifest.Root.Flags = []pluginsdk.FlagSpec{{Name: "same", Type: "bool", Default: "false"}, {Name: "same", Type: "bool", Default: "false"}}
	}}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			r := release()
			change(&r)
			_, err := ParseCatalogue(encoded(t, Catalogue{1, []Release{r}}), "auth", reference())
			if (err == nil) != (name == "valid") {
				t.Fatalf("got %v", err)
			}
		})
	}
	for _, data := range [][]byte{[]byte(`{"schemaVersion":1,"schemaVersion":1,"releases":[]}`), []byte(`{"schemaVersion":2,"plugins":{}}`), []byte(`schemaVersion: 1`), encoded(t, Catalogue{1, []Release{release(), release()}}), encoded(t, Catalogue{2, []Release{release()}}), []byte(`{"schemaVersion":1,"releases":[],"catalogue":"https://example.org"}`)} {
		if _, err := ParseCatalogue(data, "auth", reference()); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
}
func TestPublicURL(t *testing.T) {
	for _, raw := range []string{"http://example.org", "https://localhost/x", "https://127.0.0.1/x", "https://10.0.0.1/x", "https://[::1]/x", "https://user:password@example.org/x", "https://example.org/x#bad", "/relative", "https://service.local/x"} {
		if PublicURL(raw) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if err := PublicURL("https://github.com/formancehq/auth/releases/download/v2.5.2/catalogue.json"); err != nil {
		t.Fatal(err)
	}
}
func TestDecoderBounds(t *testing.T) {
	var target Index
	for _, data := range [][]byte{[]byte(strings.Repeat("a", int(MaxCatalogueBytes)+1)), []byte(strings.Repeat("[", 70) + strings.Repeat("]", 70)), []byte("plugins: {1: value}"), []byte("["), []byte("plugins: !!binary AA==")} {
		if StrictDecode(data, &target, true) == nil {
			t.Fatal("accepted malformed document")
		}
	}
}
