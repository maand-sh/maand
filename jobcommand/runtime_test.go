package jobcommand

import (
	"errors"
	"os"
	"path"
	"testing"

	"maand/bucket"
)

func TestResolveCommandScriptPython(t *testing.T) {
	dir := t.TempDir()
	command := "command_health_check"
	requireWrite(t, path.Join(dir, command+".py"), "")

	runtime, scriptPath, err := ResolveCommandScript(dir, command)
	if err != nil {
		t.Fatal(err)
	}
	if runtime != RuntimePython {
		t.Fatalf("runtime %q, want python", runtime)
	}
	if scriptPath != path.Join(dir, command+".py") {
		t.Fatalf("script %q", scriptPath)
	}
}

func TestResolveCommandScriptBunTypeScript(t *testing.T) {
	dir := t.TempDir()
	command := "command_deploy"
	requireWrite(t, path.Join(dir, command+".ts"), "export {};\n")

	runtime, scriptPath, err := ResolveCommandScript(dir, command)
	if err != nil {
		t.Fatal(err)
	}
	if runtime != RuntimeBun {
		t.Fatalf("runtime %q, want bun", runtime)
	}
	if scriptPath != path.Join(dir, command+".ts") {
		t.Fatalf("script %q", scriptPath)
	}
}

func TestResolveCommandScriptBunJavaScript(t *testing.T) {
	dir := t.TempDir()
	command := "command_health_check"
	requireWrite(t, path.Join(dir, command+".js"), "export {};\n")

	runtime, scriptPath, err := ResolveCommandScript(dir, command)
	if err != nil {
		t.Fatal(err)
	}
	if runtime != RuntimeBun {
		t.Fatalf("runtime %q, want bun", runtime)
	}
	if scriptPath != path.Join(dir, command+".js") {
		t.Fatalf("script %q", scriptPath)
	}
}

func TestResolveCommandScriptAmbiguous(t *testing.T) {
	dir := t.TempDir()
	command := "command_x"
	requireWrite(t, path.Join(dir, command+".py"), "")
	requireWrite(t, path.Join(dir, command+".ts"), "")

	_, _, err := ResolveCommandScript(dir, command)
	if !errors.Is(err, bucket.ErrInvalidJobCommandConfiguration) {
		t.Fatalf("got %v", err)
	}
}

func TestResolveCommandScriptAmbiguousPythonAndJS(t *testing.T) {
	dir := t.TempDir()
	command := "command_x"
	requireWrite(t, path.Join(dir, command+".py"), "")
	requireWrite(t, path.Join(dir, command+".js"), "")

	_, _, err := ResolveCommandScript(dir, command)
	if !errors.Is(err, bucket.ErrInvalidJobCommandConfiguration) {
		t.Fatalf("got %v", err)
	}
}

func TestResolveCommandScriptMissing(t *testing.T) {
	_, _, err := ResolveCommandScript(t.TempDir(), "command_missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCommandExecLinesBun(t *testing.T) {
	lines := CommandExecLines("/modules", "/modules/command_x.ts", RuntimeBun, "api")
	if len(lines) != 2 || lines[1] != "bun run command_x.ts" {
		t.Fatalf("lines: %#v", lines)
	}
}

func TestCommandExecLinesPython(t *testing.T) {
	lines := CommandExecLines("/modules", "/modules/command_x.py", RuntimePython, "api")
	if len(lines) != 2 || lines[1] != "python3 command_x.py" {
		t.Fatalf("lines: %#v", lines)
	}
}

func TestCommandExecLinesUnknownRuntimeDefaultsPython(t *testing.T) {
	lines := CommandExecLines("/modules", "/modules/command_x.py", Runtime("ruby"), "api")
	if lines[1] != "python3 command_x.py" {
		t.Fatalf("lines: %#v", lines)
	}
}

func requireWrite(t *testing.T, filePath, content string) {
	t.Helper()
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveCommandScriptErrors(t *testing.T) {
	_, _, err := ResolveCommandScript(t.TempDir(), "command_x")
	if !errors.Is(err, bucket.ErrJobCommandFileNotFound) {
		t.Fatalf("got %v", err)
	}
}
