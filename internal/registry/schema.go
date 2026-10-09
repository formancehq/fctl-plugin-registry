// Package registry validates the public index and its immutable product catalogues.
package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/formancehq/fctl/pkg/pluginsdk"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
)

const MaxCatalogueBytes int64 = 8 << 20

var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+].*)?$`)
var servicePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
var repositoryPattern = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)+$`)

type Index struct {
	SchemaVersion int                `json:"schemaVersion"`
	Plugins       map[string]Product `json:"plugins"`
}
type Product struct {
	Releases []Reference `json:"releases"`
}
type Reference struct {
	ServiceVersion string `json:"serviceVersion"`
	Catalogue      string `json:"catalogue"`
	SHA256         string `json:"sha256"`
}
type Catalogue struct {
	SchemaVersion int       `json:"schemaVersion"`
	Releases      []Release `json:"releases"`
}
type Release struct {
	Service        string             `json:"service"`
	ServiceVersion string             `json:"serviceVersion"`
	Revision       int                `json:"revision"`
	Platform       Platform           `json:"platform"`
	Artifact       Artifact           `json:"artifact"`
	SHA256         string             `json:"sha256"`
	Manifest       pluginsdk.Manifest `json:"manifest"`
}
type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}
type Artifact struct {
	Registry   string `json:"registry"`
	Repository string `json:"repository"`
	Digest     string `json:"digest"`
}

// StrictDecode rejects duplicate keys, unknown fields, aliases and extra documents.
// YAML is accepted only for the central index; product catalogues must be JSON.
func StrictDecode(data []byte, target any, yamlAllowed bool) error {
	if int64(len(data)) > MaxCatalogueBytes {
		return fmt.Errorf("document exceeds 8 MiB")
	}
	if !yamlAllowed && !json.Valid(data) {
		return fmt.Errorf("product catalogue must be JSON")
	}
	var node yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&node); err != nil {
		return err
	}
	if err := checkNode(&node, 0); err != nil {
		return err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected one document")
	}
	var value any
	if err := node.Decode(&value); err != nil {
		return err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(canonical))
	dec.DisallowUnknownFields()
	return dec.Decode(target)
}
func checkNode(n *yaml.Node, depth int) error {
	if depth > 64 {
		return fmt.Errorf("document nesting exceeds 64")
	}
	if n.Kind == yaml.AliasNode || n.Anchor != "" {
		return fmt.Errorf("YAML aliases and anchors are unsupported")
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			key := n.Content[i]
			if key.Tag != "!!str" {
				return fmt.Errorf("keys must be strings")
			}
			if seen[key.Value] {
				return fmt.Errorf("duplicate field %q", key.Value)
			}
			seen[key.Value] = true
		}
	}
	for _, child := range n.Content {
		if err := checkNode(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}
func PublicURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("expected public absolute HTTPS URL")
	}
	host := strings.ToLower(u.Hostname())
	ip := net.ParseIP(host)
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || (ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsMulticast())) {
		return fmt.Errorf("URL must be public")
	}
	return nil
}
func ParseIndex(data []byte) (Index, error) {
	var index Index
	if err := StrictDecode(data, &index, true); err != nil {
		return index, err
	}
	if index.SchemaVersion != 2 || index.Plugins == nil {
		return index, fmt.Errorf("expected schemaVersion 2 and plugins map")
	}
	for service, product := range index.Plugins {
		if !servicePattern.MatchString(service) || len(product.Releases) == 0 {
			return index, fmt.Errorf("invalid product %q", service)
		}
		seen := map[string]bool{}
		for _, ref := range product.Releases {
			if len(ref.ServiceVersion) > 128 || !versionPattern.MatchString(ref.ServiceVersion) || strings.HasPrefix(ref.ServiceVersion, "v") || !semver.IsValid("v"+ref.ServiceVersion) || seen[ref.ServiceVersion] {
				return index, fmt.Errorf("invalid or duplicate %s version %q", service, ref.ServiceVersion)
			}
			seen[ref.ServiceVersion] = true
			if !hashPattern.MatchString(ref.SHA256) {
				return index, fmt.Errorf("invalid catalogue sha256")
			}
			if err := PublicURL(ref.Catalogue); err != nil {
				return index, err
			}
		}
	}
	return index, nil
}
func ParseCatalogue(data []byte, service string, ref Reference) (Catalogue, error) {
	var catalogue Catalogue
	if err := StrictDecode(data, &catalogue, false); err != nil {
		return catalogue, err
	}
	if catalogue.SchemaVersion != 1 || len(catalogue.Releases) == 0 {
		return catalogue, fmt.Errorf("expected nonempty product schemaVersion 1 catalogue")
	}
	seen := map[string]bool{}
	for _, release := range catalogue.Releases {
		if err := validateRelease(release, service, ref.ServiceVersion); err != nil {
			return catalogue, err
		}
		key := fmt.Sprintf("%d/%s/%s", release.Revision, release.Platform.OS, release.Platform.Arch)
		if seen[key] {
			return catalogue, fmt.Errorf("duplicate release tuple %s", key)
		}
		seen[key] = true
	}
	return catalogue, nil
}
func validateRelease(r Release, service, version string) error {
	if r.Service != service || r.ServiceVersion != version || r.Revision < 1 {
		return fmt.Errorf("release identity does not match %s %s", service, version)
	}
	if (r.Platform.OS != "darwin" && r.Platform.OS != "linux" && r.Platform.OS != "windows") || (r.Platform.Arch != "arm64" && r.Platform.Arch != "amd64") {
		return fmt.Errorf("unsupported platform")
	}
	if !hashPattern.MatchString(r.SHA256) || !strings.HasPrefix(r.Artifact.Digest, "sha256:") || !hashPattern.MatchString(strings.TrimPrefix(r.Artifact.Digest, "sha256:")) {
		return fmt.Errorf("invalid executable or OCI digest")
	}
	if err := PublicURL(r.Artifact.Registry); err != nil {
		return err
	}
	u, _ := url.Parse(r.Artifact.Registry)
	if u.RawQuery != "" || (u.Path != "" && u.Path != "/") || !repositoryPattern.MatchString(r.Artifact.Repository) {
		return fmt.Errorf("invalid OCI origin or repository")
	}
	m := r.Manifest
	if m.Name != service || m.Service != service || m.Version != version || m.ProtocolVersion != pluginsdk.ProtocolVersion || pluginsdk.CommandName(m.Root) != service {
		return fmt.Errorf("SDK manifest identity or protocol mismatch")
	}
	if coreRoots[m.Name] {
		return fmt.Errorf("reserved plugin root %q", m.Name)
	}
	return validateCommand(m.Root, validationScope{names: map[string]bool{}, shorts: map[string]bool{"h": true, "o": true, "p": true}})
}
