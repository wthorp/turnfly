// Package containerimage builds and pushes container images through the Docker
// Engine API.
package containerimage

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultDockerHost = "unix:///var/run/docker.sock"
	flyRegistry       = "registry.fly.io"
)

// DockerPublisher builds and pushes images using a Docker Engine API endpoint.
type DockerPublisher struct {
	baseURL    string
	httpClient *http.Client
}

// PublishConfig configures a Docker build and registry push.
type PublishConfig struct {
	ContextDir string
	Dockerfile string
	Image      string
	Token      string
	Output     io.Writer
}

// NewDockerPublisher creates a Docker Engine API publisher. Only unix sockets
// are supported because that is Docker Desktop and dockerd's common local API.
func NewDockerPublisher(dockerHost string) (*DockerPublisher, error) {
	if dockerHost == "" {
		dockerHost = os.Getenv("DOCKER_HOST")
	}
	if dockerHost == "" {
		dockerHost = defaultDockerHost
	}
	if !strings.HasPrefix(dockerHost, "unix://") {
		return nil, fmt.Errorf("unsupported DOCKER_HOST %q: only unix:// sockets are supported", dockerHost)
	}

	socketPath := strings.TrimPrefix(dockerHost, "unix://")
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}

	return &DockerPublisher{
		baseURL: "http://docker",
		httpClient: &http.Client{
			Transport: transport,
		},
	}, nil
}

// BuildAndPush builds the configured image and pushes it to registry.fly.io.
func (p *DockerPublisher) BuildAndPush(ctx context.Context, cfg PublishConfig) error {
	if cfg.ContextDir == "" {
		cfg.ContextDir = "."
	}
	if cfg.Dockerfile == "" {
		cfg.Dockerfile = "Dockerfile"
	}
	if cfg.Image == "" {
		return errors.New("image is required")
	}
	if cfg.Token == "" {
		return errors.New("Fly API token is required for registry push")
	}
	if !strings.HasPrefix(cfg.Image, flyRegistry+"/") {
		return fmt.Errorf("image must be in %s, got %q", flyRegistry, cfg.Image)
	}

	contextTar, err := tarBuildContext(cfg.ContextDir)
	if err != nil {
		return fmt.Errorf("create docker build context: %w", err)
	}

	if err := p.build(ctx, cfg, bytes.NewReader(contextTar)); err != nil {
		return err
	}
	if err := p.push(ctx, cfg); err != nil {
		return err
	}
	return nil
}

func (p *DockerPublisher) build(ctx context.Context, cfg PublishConfig, body io.Reader) error {
	query := url.Values{}
	query.Set("t", cfg.Image)
	query.Set("dockerfile", cfg.Dockerfile)
	query.Set("rm", "1")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/build?"+query.Encode(), body)
	if err != nil {
		return fmt.Errorf("create docker build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-tar")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("docker build request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker build failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if err := copyDockerStream(cfg.Output, resp.Body); err != nil {
		return fmt.Errorf("read docker build stream: %w", err)
	}
	return nil
}

func (p *DockerPublisher) push(ctx context.Context, cfg PublishConfig) error {
	name, tag := splitImageTag(cfg.Image)
	authHeader, err := registryAuthHeader(cfg.Token)
	if err != nil {
		return err
	}

	query := url.Values{}
	query.Set("tag", tag)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/images/"+name+"/push?"+query.Encode(), nil)
	if err != nil {
		return fmt.Errorf("create docker push request: %w", err)
	}
	req.Header.Set("X-Registry-Auth", authHeader)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("docker push request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker push failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if err := copyDockerStream(cfg.Output, resp.Body); err != nil {
		return fmt.Errorf("read docker push stream: %w", err)
	}
	return nil
}

func splitImageTag(image string) (name, tag string) {
	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon > lastSlash {
		return image[:lastColon], image[lastColon+1:]
	}
	return image, "latest"
}

func registryAuthHeader(token string) (string, error) {
	auth := map[string]string{
		"username":      "x",
		"password":      token,
		"serveraddress": flyRegistry,
	}
	data, err := json.Marshal(auth)
	if err != nil {
		return "", fmt.Errorf("marshal registry auth: %w", err)
	}
	return base64.URLEncoding.EncodeToString(data), nil
}

func copyDockerStream(output io.Writer, input io.Reader) error {
	if output == nil {
		output = io.Discard
	}

	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		var msg struct {
			Stream string `json:"stream"`
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		if err := json.Unmarshal(line, &msg); err == nil {
			if msg.Error != "" {
				return errors.New(msg.Error)
			}
			switch {
			case msg.Stream != "":
				fmt.Fprint(output, msg.Stream)
			case msg.Status != "":
				fmt.Fprintln(output, msg.Status)
			}
			continue
		}
		fmt.Fprintln(output, string(line))
	}
	return scanner.Err()
}

func tarBuildContext(root string) ([]byte, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	defer tw.Close()

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if shouldSkipContextPath(rel, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func shouldSkipContextPath(rel string, d fs.DirEntry) bool {
	name := d.Name()
	if name == ".git" || name == "turnfly" || name == ".env" {
		return true
	}
	if strings.HasSuffix(name, ".test") || strings.HasSuffix(name, ".out") || strings.HasSuffix(name, ".coverprofile") {
		return true
	}
	if strings.HasPrefix(rel, "coverage.") || rel == "profile.cov" {
		return true
	}
	return false
}
