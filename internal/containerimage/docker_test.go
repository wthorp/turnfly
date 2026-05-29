package containerimage

import (
	"archive/tar"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestSplitImageTag(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		wantName string
		wantTag  string
	}{
		{
			name:     "explicit tag",
			image:    "registry.fly.io/myapp:abc123",
			wantName: "registry.fly.io/myapp",
			wantTag:  "abc123",
		},
		{
			name:     "default tag",
			image:    "registry.fly.io/myapp",
			wantName: "registry.fly.io/myapp",
			wantTag:  "latest",
		},
		{
			name:     "registry port is not tag",
			image:    "localhost:5000/myapp",
			wantName: "localhost:5000/myapp",
			wantTag:  "latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotTag := splitImageTag(tt.image)
			if gotName != tt.wantName || gotTag != tt.wantTag {
				t.Fatalf("splitImageTag() = %q, %q; want %q, %q", gotName, gotTag, tt.wantName, tt.wantTag)
			}
		})
	}
}

func TestRegistryAuthHeader(t *testing.T) {
	header, err := registryAuthHeader("fly-token")
	if err != nil {
		t.Fatalf("registryAuthHeader() error = %v", err)
	}

	data, err := base64.URLEncoding.DecodeString(header)
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}

	var auth map[string]string
	if err := json.Unmarshal(data, &auth); err != nil {
		t.Fatalf("unmarshal auth: %v", err)
	}
	if auth["username"] != "x" || auth["password"] != "fly-token" || auth["serveraddress"] != flyRegistry {
		t.Fatalf("unexpected auth config: %#v", auth)
	}
}

func TestTarBuildContextSkipsLocalArtifacts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Dockerfile"), "FROM scratch\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/test\n")
	writeFile(t, filepath.Join(dir, "turnfly"), "binary")
	writeFile(t, filepath.Join(dir, ".env"), "SECRET=value\n")
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	writeFile(t, filepath.Join(dir, ".git", "HEAD"), "ref: refs/heads/main\n")

	data, err := tarBuildContext(dir)
	if err != nil {
		t.Fatalf("tarBuildContext() error = %v", err)
	}

	got := tarNames(t, data)
	if !got["Dockerfile"] || !got["go.mod"] {
		t.Fatalf("expected Dockerfile and go.mod in context, got %#v", got)
	}
	for _, name := range []string{"turnfly", ".env", ".git", ".git/HEAD"} {
		if got[name] {
			t.Fatalf("expected %q to be skipped, got %#v", name, got)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func tarNames(t *testing.T, data []byte) map[string]bool {
	t.Helper()
	tr := tar.NewReader(bytes.NewReader(data))
	names := make(map[string]bool)
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("read tar: %v", err)
		}
		names[header.Name] = true
	}
	return names
}
