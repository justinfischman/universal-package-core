package python

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

// TestAppKindTemplatesMatchTheScriptCommands requires each app kind's templates
// to contain the file its dev and start scripts run, and no service files.
func TestAppKindTemplatesMatchTheScriptCommands(t *testing.T) {
	for kind, want := range map[string][]string{
		"app-cli":     {".gitignore", "APP.md.tmpl", "cli.py"},
		"app-library": {".gitignore", "APP.md.tmpl", "demo.py", "lib.py"},
	} {
		files, ok := New().Templates(kind)
		if !ok {
			t.Fatalf("python must provide %s templates", kind)
		}
		var got []string
		err := fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				got = append(got, p)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s templates = %v, want %v", kind, got, want)
		}
	}
}

func TestAppTemplatesGolden(t *testing.T) {
	files, ok := New().Templates("app")
	if !ok {
		t.Fatal("python must provide app templates")
	}
	var got []string
	err := fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			got = append(got, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".gitignore", "APP.md.tmpl", "Dockerfile", "compose.yaml", "server.py"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("app templates = %v, want %v", got, want)
	}
	gitignore, _ := fs.ReadFile(files, ".gitignore")
	if string(gitignore) != ".venv/\n__pycache__/\n.universal-package-core/run/\n" {
		t.Errorf(".gitignore = %q", gitignore)
	}
	server, _ := fs.ReadFile(files, "server.py")
	for _, need := range []string{`"127.0.0.1"`, "APP_HOST", "sys.argv[1]", "else 8000"} {
		if !strings.Contains(string(server), need) {
			t.Errorf("server.py lacks %s", need)
		}
	}
	assertContainerFiles(t, files, "python", "server.py")
}

// assertContainerFiles checks the optional Dockerfile and compose.yaml: the
// container listens on all interfaces, the compose file publishes the app on
// the loopback address only, and the image starts the app.
func assertContainerFiles(t *testing.T, files fs.FS, command ...string) {
	t.Helper()
	docker, _ := fs.ReadFile(files, "Dockerfile")
	for _, need := range []string{"ENV APP_HOST=0.0.0.0", "EXPOSE 8000", "CMD ["} {
		if !strings.Contains(string(docker), need) {
			t.Errorf("Dockerfile lacks %s:\n%s", need, docker)
		}
	}
	compose, _ := fs.ReadFile(files, "compose.yaml")
	for _, need := range []string{"services:", "build: .", `"127.0.0.1:8000:8000"`} {
		if !strings.Contains(string(compose), need) {
			t.Errorf("compose.yaml lacks %s:\n%s", need, compose)
		}
	}
	for _, part := range command {
		if !strings.Contains(string(docker), `"`+part+`"`) {
			t.Errorf("Dockerfile CMD lacks %q:\n%s", part, docker)
		}
	}
}
