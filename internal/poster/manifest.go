package poster

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxPosterImageBytes = 10 * 1024 * 1024

type Manifest struct {
	Posters map[string]PosterAsset `json:"posters" yaml:"posters"`
}

type PosterAsset struct {
	Src            string `json:"src" yaml:"src"`
	Source         string `json:"source,omitempty" yaml:"source,omitempty"`
	Alias          string `json:"alias,omitempty" yaml:"alias,omitempty"`
	MatchedTitle   string `json:"matched_title,omitempty" yaml:"matched_title,omitempty"`
	Confidence     string `json:"confidence,omitempty" yaml:"confidence,omitempty"`
	Note           string `json:"note,omitempty" yaml:"note,omitempty"`
	ReviewRequired bool   `json:"review_required,omitempty" yaml:"review_required,omitempty"`
}

func (a *PosterAsset) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		a.Src = strings.TrimSpace(value.Value)
		return nil
	}
	type alias PosterAsset
	var decoded alias
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	*a = PosterAsset(decoded)
	return nil
}

func (a *PosterAsset) UnmarshalJSON(data []byte) error {
	var src string
	if err := json.Unmarshal(data, &src); err == nil {
		a.Src = strings.TrimSpace(src)
		return nil
	}
	type alias PosterAsset
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*a = PosterAsset(decoded)
	return nil
}

func LoadManifest(path string) (Manifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read poster manifest: %w", err)
	}
	var manifest Manifest
	trimmed := strings.TrimSpace(string(content))
	if strings.HasPrefix(trimmed, "{") {
		err = json.Unmarshal(content, &manifest)
	} else {
		err = yaml.Unmarshal(content, &manifest)
	}
	if err != nil {
		return Manifest{}, fmt.Errorf("parse poster manifest: %w", err)
	}
	if manifest.Posters == nil {
		manifest.Posters = map[string]PosterAsset{}
	}
	return manifest, nil
}

func (m Manifest) Find(title string) (PosterAsset, bool) {
	key := normalizeTitle(title)
	if key == "" {
		return PosterAsset{}, false
	}
	for name, asset := range m.Posters {
		if normalizeTitle(name) == key || normalizeTitle(asset.Alias) == key || normalizeTitle(asset.MatchedTitle) == key {
			return asset, strings.TrimSpace(asset.Src) != ""
		}
	}
	return PosterAsset{}, false
}

func (m Manifest) Merge(overlay Manifest) Manifest {
	merged := Manifest{Posters: map[string]PosterAsset{}}
	for name, asset := range m.Posters {
		merged.Posters[name] = asset
	}
	for name, asset := range overlay.Posters {
		merged.Posters[name] = asset
	}
	return merged
}

func MaterializeSrc(src string, baseDir string) (string, error) {
	trimmed := strings.TrimSpace(src)
	if trimmed == "" {
		return "", fmt.Errorf("poster src is empty")
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") || strings.HasPrefix(trimmed, "data:image/") {
		return trimmed, nil
	}
	path := expandHome(trimmed)
	if !filepath.IsAbs(path) {
		if strings.TrimSpace(baseDir) != "" {
			path = filepath.Join(baseDir, path)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("read poster image %q: %w", trimmed, err)
	}
	if info.Size() > maxPosterImageBytes {
		return "", fmt.Errorf("poster image %q exceeds %d bytes", trimmed, maxPosterImageBytes)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read poster image %q: %w", trimmed, err)
	}
	mime := http.DetectContentType(content)
	if !strings.HasPrefix(mime, "image/") {
		mime = imageMimeFromExt(filepath.Ext(path))
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(content), nil
}

func normalizeTitle(title string) string {
	trimmed := strings.TrimSpace(title)
	trimmed = strings.TrimPrefix(trimmed, "《")
	trimmed = strings.TrimSuffix(trimmed, "》")
	trimmed = strings.Trim(trimmed, " \t\r\n\"'`*_：:")
	return strings.ToLower(trimmed)
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func imageMimeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "image/png"
	}
}
