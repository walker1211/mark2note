package readiness

import (
	"os"
	"strings"
	"testing"
)

const projectName = "mark2note"

var releaseSuffixes = []string{"darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64"}

func TestGitHubCIWorkflowReadiness(t *testing.T) {
	content := readTextFile(t, ".github/workflows/ci.yml")
	for _, want := range []string{
		"name: CI",
		"push:",
		"pull_request:",
		"actions/checkout@v6",
		"fetch-depth: 0",
		"actions/setup-go@v6",
		"go-version-file: go.mod",
		"scripts/secret-scan.sh",
		"gofmt -l",
		"go vet ./...",
		"go test ./...",
	} {
		assertContains(t, content, want)
	}
}

func TestGitHubReleaseWorkflowReadiness(t *testing.T) {
	content := readTextFile(t, ".github/workflows/release.yml")
	for _, want := range []string{
		"name: Release",
		"tags:",
		"v*",
		"contents: write",
		"preflight:",
		"build:",
		"release:",
		"needs: preflight",
		"needs: build",
		"actions/checkout@v6",
		"fetch-depth: 0",
		"actions/setup-go@v6",
		"go-version-file: go.mod",
		"scripts/ci-local.sh clean",
		"package=\"" + projectName + "-${SUFFIX}\"",
		"actions/upload-artifact@v7",
		"actions/download-artifact@v8",
		"sha256sum",
		"checksums.txt",
		"gh release view",
		"gh release upload",
		"gh release create",
		"GH_TOKEN",
	} {
		assertContains(t, content, want)
	}
	for _, suffix := range releaseSuffixes {
		assertContains(t, content, suffix)
	}
	for _, forbidden := range []string{
		"secrets.",
		"softprops/action-gh-release",
		"GoReleaser",
		"goreleaser",
		"git push",
		"git tag",
	} {
		assertNotContains(t, content, forbidden)
	}
}

func TestGitHubCodeQLWorkflowReadiness(t *testing.T) {
	content := readTextFile(t, ".github/workflows/codeql.yml")
	for _, want := range []string{
		"name: CodeQL",
		"push:",
		"pull_request:",
		"schedule:",
		"security-events: write",
		"actions/checkout@v6",
		"github/codeql-action/init@v4",
		"languages: go",
		"github/codeql-action/autobuild@v4",
		"github/codeql-action/analyze@v4",
	} {
		assertContains(t, content, want)
	}
}

func TestGitHubReadinessScriptUsesDedicatedEndpoints(t *testing.T) {
	path := "scripts/github-readiness.sh"
	content := readTextFile(t, path)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("expected %s to be executable", path)
	}
	for _, want := range []string{
		"gh api \"repos/${repo}\"",
		"security_and_analysis.secret_scanning.status",
		"security_and_analysis.secret_scanning_push_protection.status",
		"gh api \"repos/${repo}/private-vulnerability-reporting\"",
		"private-vulnerability-reporting",
		"branches/${default_branch}/protection",
		"code-scanning/analyses",
	} {
		assertContains(t, content, want)
	}
	assertNotContains(t, content, "security_and_analysis.private")
}

func readTextFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
	return string(content)
}

func assertContains(t *testing.T, content, want string) {
	t.Helper()
	if !strings.Contains(content, want) {
		t.Fatalf("expected content to contain %q", want)
	}
}

func assertNotContains(t *testing.T, content, forbidden string) {
	t.Helper()
	if strings.Contains(content, forbidden) {
		t.Fatalf("expected content not to contain %q", forbidden)
	}
}
