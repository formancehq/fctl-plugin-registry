package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func sum(data []byte) string                                          { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func TestLiveGateWithAnonymousOCI(t *testing.T) {
	cases := []string{"valid", "no challenge", "catalogue hash", "catalogue identity", "manifest hash", "wrong layer media", "wrong layer hash", "wrong config", "binary hash", "binary size", "catalogue HTTP", "manifest HTTP", "token HTTP", "token missing", "bad scope", "bad realm", "duplicate challenge", "blob HTTP", "binary read failure"}
	for _, mode := range cases {
		t.Run(mode, func(t *testing.T) {
			binary := []byte("synthetic executable bytes")
			r := release()
			r.Artifact.Registry = "https://fixture.example"
			r.SHA256 = sum(binary)
			config := []byte("{}")
			if mode == "wrong config" {
				config = []byte("[]")
			}
			layer := descriptor{MediaType: binaryMedia, Digest: "sha256:" + r.SHA256, Size: int64(len(binary))}
			if mode == "wrong layer media" {
				layer.MediaType = "application/tar"
			}
			if mode == "wrong layer hash" {
				layer.Digest = "sha256:" + hash("a")
			}
			if mode == "binary size" {
				layer.Size++
			}
			manifest := imageManifest{SchemaVersion: 2, MediaType: manifestMedia, ArtifactType: artifactMedia, Config: descriptor{MediaType: configMedia, Digest: "sha256:" + sum(config), Size: int64(len(config))}, Layers: []descriptor{layer}}
			mb := encoded(t, manifest)
			r.Artifact.Digest = "sha256:" + sum(mb)
			if mode == "catalogue identity" {
				r.Service = "ledger"
			}
			cb := encoded(t, Catalogue{1, []Release{r}})
			ref := reference()
			ref.Catalogue = "https://fixture.example/catalogue"
			ref.SHA256 = sum(cb)
			if mode == "catalogue hash" {
				ref.SHA256 = hash("a")
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				switch {
				case req.URL.Path == "/catalogue":
					if mode == "catalogue HTTP" {
						w.WriteHeader(404)
						return
					}
					_, _ = w.Write(cb)
				case req.URL.Path == "/token":
					if mode == "token HTTP" {
						w.WriteHeader(403)
						return
					}
					if mode == "token missing" {
						fmt.Fprint(w, `{}`)
						return
					}
					if req.URL.Query().Get("scope") != "repository:"+r.Artifact.Repository+":pull" {
						t.Error("incorrect scope")
					}
					fmt.Fprint(w, `{"token":"anonymous-fixture-token"}`)
				case strings.Contains(req.URL.Path, "/manifests/"):
					if mode == "manifest HTTP" {
						w.WriteHeader(404)
						return
					}
					if mode != "no challenge" && req.Header.Get("Authorization") == "" {
						realm := "https://fixture.example/token"
						scope := "repository:" + r.Artifact.Repository + ":pull"
						if mode == "bad scope" {
							scope = "repository:other:push"
						}
						if mode == "bad realm" {
							realm = "http://localhost/token"
						}
						challenge := fmt.Sprintf(`Bearer realm="%s",service="fixture",scope="%s"`, realm, scope)
						if mode == "duplicate challenge" {
							challenge += `,scope="bad"`
						}
						w.Header().Set("WWW-Authenticate", challenge)
						w.WriteHeader(401)
						return
					}
					if mode == "manifest hash" {
						_, _ = w.Write(append(mb, ' '))
						return
					}
					_, _ = w.Write(mb)
				case strings.HasSuffix(req.URL.Path, manifest.Config.Digest):
					_, _ = w.Write(config)
				default:
					if mode == "blob HTTP" {
						w.WriteHeader(403)
						return
					}
					if mode == "binary hash" {
						binary[0] = '!'
					}
					_, _ = w.Write(binary)
				}
			}))
			defer server.Close()
			destination, _ := url.Parse(server.URL)
			client := server.Client()
			base := client.Transport
			client.Transport = roundTrip(func(req *http.Request) (*http.Response, error) {
				clone := req.Clone(req.Context())
				u := *req.URL
				u.Scheme = destination.Scheme
				u.Host = destination.Host
				clone.URL = &u
				if mode == "binary read failure" && strings.HasSuffix(u.Path, "sha256:"+r.SHA256) {
					return nil, fmt.Errorf("network failed")
				}
				return base.RoundTrip(clone)
			})
			err := (Validator{Client: client}).Validate(context.Background(), Index{2, map[string]Product{"auth": {[]Reference{ref}}}})
			wantOK := mode == "valid" || mode == "no challenge"
			if (err == nil) != wantOK {
				t.Fatalf("got %v", err)
			}
		})
	}
}
func TestResponseBounds(t *testing.T) {
	for _, test := range []struct {
		status int
		body   string
		limit  int64
	}{{200, "12345", 4}, {404, "", 4}} {
		resp := &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body))}
		if _, err := readResponse(resp, test.limit); err == nil {
			t.Fatal("accepted response")
		}
	}
}

func TestRedirectPolicy(t *testing.T) {
	for _, mode := range []string{"cross host", "insecure", "loop"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Host == "cdn.example" {
					if r.Header.Get("Authorization") != "" {
						t.Error("registry bearer token forwarded across hosts")
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Header: http.Header{}, Request: r}, nil
				}
				target := "https://cdn.example/blob"
				if mode == "insecure" {
					target = "http://cdn.example/blob"
				}
				if mode == "loop" {
					target = "https://registry.example/again"
				}
				return &http.Response{StatusCode: 302, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{"Location": []string{target}}, Request: r}, nil
			})}
			data, err := (Validator{Client: client}).read(context.Background(), "https://registry.example/blob", "registry-token", 10)
			if mode == "cross host" {
				if err != nil || string(data) != "ok" {
					t.Fatalf("data %s, %v", data, err)
				}
			} else if err == nil {
				t.Fatal("unsafe redirects accepted")
			}
			if calls > 7 {
				t.Fatal("unbounded redirects")
			}
		})
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) { return nil, r.Context().Err() })}
	if _, err := (Validator{Client: client}).read(ctx, "https://example.org", "", 10); err == nil {
		t.Fatal("cancellation ignored")
	}
}
