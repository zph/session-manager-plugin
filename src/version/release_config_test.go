package version

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"text/template"

	"gopkg.in/yaml.v3"
)

const versionLdflag = "github.com/zph/session-manager-plugin/src/version.Version="

type goreleaserConfig struct {
	Builds []goreleaserBuild `yaml:"builds"`
}

type goreleaserBuild struct {
	ID      string   `yaml:"id"`
	Ldflags []string `yaml:"ldflags"`
}

func TestGoReleaserEmbedsAWSCompatibleReleaseClientVersion(t *testing.T) {
	config := readGoReleaserConfig(t)
	data := map[string]string{
		"Version":     "0.0.0-1.2.694.8",
		"ShortCommit": "abcdef1",
	}

	for _, buildID := range []string{"session-manager-plugin", "ssmcli", "ssm-port-forward"} {
		ldflag := renderVersionLdflag(t, config, buildID, data)
		if !strings.Contains(ldflag, versionLdflag+"1.2.694.8") {
			t.Fatalf("%s embeds incompatible release client version: %q", buildID, ldflag)
		}
		if strings.Contains(ldflag, versionLdflag+"0.0.0-") {
			t.Fatalf("%s embeds GoReleaser artifact version instead of AWS client version: %q", buildID, ldflag)
		}
	}
}

func TestGoReleaserSnapshotClientVersionFallsBackToDev(t *testing.T) {
	config := readGoReleaserConfig(t)
	data := map[string]string{
		"Version":     "0.0.0-next",
		"ShortCommit": "abcdef1",
	}

	for _, buildID := range []string{"session-manager-plugin", "ssmcli", "ssm-port-forward"} {
		ldflag := renderVersionLdflag(t, config, buildID, data)
		if !strings.Contains(ldflag, versionLdflag+"dev") {
			t.Fatalf("%s embeds incompatible snapshot client version: %q", buildID, ldflag)
		}
	}
}

func readGoReleaserConfig(t *testing.T) goreleaserConfig {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate test file")
	}

	path := filepath.Join(filepath.Dir(filename), "..", "..", ".goreleaser.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var config goreleaserConfig
	if err := yaml.Unmarshal(content, &config); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return config
}

func renderVersionLdflag(t *testing.T, config goreleaserConfig, buildID string, data map[string]string) string {
	t.Helper()

	for _, build := range config.Builds {
		if build.ID != buildID {
			continue
		}
		for _, ldflag := range build.Ldflags {
			if strings.Contains(ldflag, versionLdflag) {
				return renderGoReleaserTemplate(t, buildID, ldflag, data)
			}
		}
		t.Fatalf("%s has no version ldflag", buildID)
	}
	t.Fatalf("goreleaser build %s not found", buildID)
	return ""
}

func renderGoReleaserTemplate(t *testing.T, name, text string, data map[string]string) string {
	t.Helper()

	tmpl, err := template.New(name).Funcs(template.FuncMap{
		"replace": strings.ReplaceAll,
	}).Parse(text)
	if err != nil {
		t.Fatalf("parse %s ldflag template: %v", name, err)
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data); err != nil {
		t.Fatalf("render %s ldflag template: %v", name, err)
	}
	return rendered.String()
}
