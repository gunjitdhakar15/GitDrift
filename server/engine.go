package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"gitdrift/pkg/analyzer"
)

var errMaxJobs = errors.New("too many jobs in flight, try again later")

// ValidateRepoURL normalizes a GitHub repository URL to its clone endpoint.
// Accepts https/http URLs and ssh-style git@github.com:owner/repo URLs.
func ValidateRepoURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("repository URL is required")
	}

	// ssh-style: git@github.com:owner/repo(.git)
	if strings.HasPrefix(raw, "git@github.com:") {
		path := strings.TrimPrefix(raw, "git@github.com:")
		path = strings.TrimSuffix(path, ".git")
		if err := validateOwnerRepo(path); err != nil {
			return "", err
		}
		return "https://github.com/" + path + ".git", nil
	}

	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "", errors.New("URL must use http(s) scheme")
	}
	host := strings.ToLower(u.Host)
	if host != "github.com" && host != "www.github.com" {
		return "", fmt.Errorf("only github.com repositories are supported, got %q", host)
	}

	path := strings.Trim(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	// strip extra segments (tree/blob/branch refs)
	segs := strings.Split(path, "/")
	if len(segs) < 2 {
		return "", errors.New("URL must contain an owner and repository name")
	}
	return "https://github.com/" + segs[0] + "/" + segs[1] + ".git", nil
}

func validateOwnerRepo(path string) error {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	if len(segs) < 2 || segs[0] == "" || segs[1] == "" {
		return errors.New("URL must contain an owner and repository name")
	}
	return nil
}

// Engine performs remote analysis: shallow clone + full analyzer run.
type Engine struct {
	CloneTimeout time.Duration
}

// Analyze clones repoURL into a temp dir, runs every analyzer, and cleans up.
func (e *Engine) Analyze(cloneURL string, depth int) (*analyzer.Report, error) {
	dir, err := os.MkdirTemp("", "gitdrift-*")
	if err != nil {
		return nil, fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	ctx, cancel := context.WithTimeout(context.Background(), e.CloneTimeout)
	defer cancel()

	if err := cloneRepository(ctx, cloneURL, dir, depth); err != nil {
		return nil, fmt.Errorf("clone failed: %w", err)
	}

	report, err := analyzer.BuildReport(dir)
	if err != nil {
		return nil, fmt.Errorf("analysis failed: %w", err)
	}
	return report, nil
}

// cloneRepository shallow-clones into the target directory.
func cloneRepository(ctx context.Context, cloneURL, dir string, depth int) error {
	args := []string{"clone", "--quiet", "--single-branch"}
	if depth > 0 {
		args = append(args, "--depth", itoa(depth))
	}
	args = append(args, cloneURL, dir)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	// Optional token for rate-limit relief / private repos.
	if token := os.Getenv("GITDRIFT_GITHUB_TOKEN"); token != "" {
		u, err := url.Parse(cloneURL)
		if err == nil {
			u.User = url.UserPassword("x-access-token", token)
			args[len(args)-2] = u.String()
			cmd = exec.CommandContext(ctx, "git", args...)
			cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		}
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
