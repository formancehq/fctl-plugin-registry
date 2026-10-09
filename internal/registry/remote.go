package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	manifestMedia        = "application/vnd.oci.image.manifest.v1+json"
	artifactMedia        = "application/vnd.formance.fctl.plugin.v1"
	binaryMedia          = "application/vnd.formance.fctl.plugin.executable.v1"
	configMedia          = "application/vnd.oci.empty.v1+json"
	maxBinaryBytes int64 = 128 << 20
)

type Validator struct{ Client *http.Client }
type descriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int64             `json:"size"`
	Annotations map[string]string `json:"annotations,omitempty"`
}
type imageManifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	MediaType     string            `json:"mediaType"`
	ArtifactType  string            `json:"artifactType"`
	Config        descriptor        `json:"config"`
	Layers        []descriptor      `json:"layers"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

// Validate verifies each immutable reference, then downloads every advertised
// executable anonymously without running it. No local or Cloud credentials are used.
func (v Validator) Validate(ctx context.Context, index Index) error {
	for service, product := range index.Plugins {
		for _, ref := range product.Releases {
			body, err := v.read(ctx, ref.Catalogue, "", MaxCatalogueBytes)
			if err != nil {
				return fmt.Errorf("%s %s catalogue: %w", service, ref.ServiceVersion, err)
			}
			if err := checkHash(body, ref.SHA256); err != nil {
				return err
			}
			cat, err := ParseCatalogue(body, service, ref)
			if err != nil {
				return err
			}
			for _, r := range cat.Releases {
				if err := v.validateArtifact(ctx, r); err != nil {
					return fmt.Errorf("%s %s %s/%s: %w", service, ref.ServiceVersion, r.Platform.OS, r.Platform.Arch, err)
				}
			}
		}
	}
	return nil
}
func checkHash(data []byte, want string) error {
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != want {
		return fmt.Errorf("SHA-256 mismatch")
	}
	return nil
}
func (v Validator) request(ctx context.Context, raw, token string) (*http.Response, error) {
	if err := PublicURL(raw); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", manifestMedia)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	// Revalidate each redirect; credentials must never cross registry origins.
	client := *v.Client
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return fmt.Errorf("too many redirects")
		}
		if err := PublicURL(next.URL.String()); err != nil {
			return err
		}
		if len(via) > 0 && next.URL.Host != via[0].URL.Host {
			next.Header.Del("Authorization")
		}
		return nil
	}
	return client.Do(req)
}
func readResponse(resp *http.Response, limit int64) ([]byte, error) {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("response exceeds size limit")
	}
	return body, nil
}
func (v Validator) read(ctx context.Context, raw, token string, limit int64) ([]byte, error) {
	resp, err := v.request(ctx, raw, token)
	if err != nil {
		return nil, err
	}
	return readResponse(resp, limit)
}

var challengeField = regexp.MustCompile(`([a-z]+)="([^"]*)"`)

func (v Validator) registryToken(ctx context.Context, base string, a Artifact) (string, error) {
	resp, err := v.request(ctx, base+"/manifests/"+a.Digest, "")
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		return "", nil
	}
	header := resp.Header.Get("WWW-Authenticate")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized || !strings.HasPrefix(header, "Bearer ") {
		return "", fmt.Errorf("unexpected OCI authentication challenge")
	}
	fields := map[string]string{}
	for _, match := range challengeField.FindAllStringSubmatch(header, -1) {
		if _, ok := fields[match[1]]; ok {
			return "", fmt.Errorf("duplicate authentication parameter")
		}
		fields[match[1]] = match[2]
	}
	if fields["realm"] == "" || fields["scope"] != "repository:"+a.Repository+":pull" {
		return "", fmt.Errorf("invalid anonymous pull challenge")
	}
	if err := PublicURL(fields["realm"]); err != nil {
		return "", err
	}
	u, _ := url.Parse(fields["realm"])
	q := u.Query()
	q.Set("service", fields["service"])
	q.Set("scope", fields["scope"])
	u.RawQuery = q.Encode()
	body, err := v.read(ctx, u.String(), "", 1<<20)
	if err != nil {
		return "", err
	}
	var answer struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return "", err
	}
	if answer.Token == "" {
		answer.Token = answer.AccessToken
	}
	if answer.Token == "" {
		return "", fmt.Errorf("anonymous token missing")
	}
	return answer.Token, nil
}
func validDescriptor(d descriptor, media string, limit int64) bool {
	return d.MediaType == media && strings.HasPrefix(d.Digest, "sha256:") && hashPattern.MatchString(strings.TrimPrefix(d.Digest, "sha256:")) && d.Size >= 0 && d.Size <= limit
}
func (v Validator) validateArtifact(ctx context.Context, r Release) error {
	base := strings.TrimRight(r.Artifact.Registry, "/") + "/v2/" + r.Artifact.Repository
	token, err := v.registryToken(ctx, base, r.Artifact)
	if err != nil {
		return err
	}
	body, err := v.read(ctx, base+"/manifests/"+r.Artifact.Digest, token, 4<<20)
	if err != nil {
		return err
	}
	if err := checkHash(body, strings.TrimPrefix(r.Artifact.Digest, "sha256:")); err != nil {
		return fmt.Errorf("OCI manifest: %w", err)
	}
	var manifest imageManifest
	if err := StrictDecode(body, &manifest, false); err != nil {
		return err
	}
	if manifest.SchemaVersion != 2 || manifest.MediaType != manifestMedia || manifest.ArtifactType != artifactMedia || len(manifest.Layers) != 1 || !validDescriptor(manifest.Config, configMedia, 1<<20) || !validDescriptor(manifest.Layers[0], binaryMedia, maxBinaryBytes) {
		return fmt.Errorf("invalid OCI plugin descriptor")
	}
	layer := manifest.Layers[0]
	if layer.Digest != "sha256:"+r.SHA256 || layer.Size == 0 {
		return fmt.Errorf("executable descriptor differs from catalogue")
	}
	config, err := v.read(ctx, base+"/blobs/"+manifest.Config.Digest, token, 1<<20)
	if err != nil {
		return err
	}
	if int64(len(config)) != manifest.Config.Size || checkHash(config, strings.TrimPrefix(manifest.Config.Digest, "sha256:")) != nil || string(config) != "{}" {
		return fmt.Errorf("invalid OCI empty config")
	}
	resp, err := v.request(ctx, base+"/blobs/"+layer.Digest, token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("executable HTTP %d", resp.StatusCode)
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(resp.Body, maxBinaryBytes+1))
	if err != nil {
		return err
	}
	if n != layer.Size || n > maxBinaryBytes || hex.EncodeToString(h.Sum(nil)) != r.SHA256 {
		return fmt.Errorf("executable size or SHA-256 mismatch")
	}
	return nil
}
