package alfred

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// UIScript handles a script action in an Alfred UI. It receives the action's
// args and returns JSON actions, such as UIActions, which Alfred applies to
// the UI. Returning nil changes nothing.
type UIScript func(args []string) (any, error)

// UIActions are JSON actions for an Alfred UI, such as
//
//	alfred.UIActions{"update": map[string]any{"ids": map[string]any{
//		"status": map[string]any{"text": "Saved"},
//	}}}
//
// See https://www.alfredapp.com/help/alfredui/actions/json/.
type UIActions map[string]any

// UINode is an Alfred UI: a native interface described by the ui.json of a
// .alfredui package.
type UINode struct {
	node[*UINode]
	path            string
	files           fs.FS
	external, stack bool
	scripts         map[string]UIScript
}

// AlfredUI shows the Alfred UI package files, packaged into the workflow as
// name, such as "clipboard.alfredui". files holds the package's contents, so
// it has ui.json at its root; from an embed.FS, use fs.Sub. The input that
// opens the UI is its first arg. Nodes connected with Then receive the args of
// the UI's finish action.
func AlfredUI(name string, files fs.FS) *UINode {
	u := &UINode{path: name, files: files}
	u.self, u.label = u, strings.TrimSuffix(path.Base(name), ".alfredui")
	return u
}

// External opens the UI in its own window instead of inside Alfred.
func (u *UINode) External() *UINode {
	u.external = true
	return u
}

// Stack makes ⎋ return to the previous Alfred view instead of closing Alfred.
func (u *UINode) Stack() *UINode {
	u.stack = true
	return u
}

// Script makes handler available to the UI's script actions as
// "scripts/<name>", relative to the package:
//
//	{"type": "script", "src": "scripts/save", "args": ["{value:editor.text}"]}
func (u *UINode) Script(name string, handler UIScript) *UINode {
	if u.scripts == nil {
		u.scripts = make(map[string]UIScript)
	}
	u.scripts[name] = handler
	return u
}

func (u *UINode) validate() error {
	clean := path.Clean(u.path)
	if !strings.HasSuffix(u.path, ".alfredui") || clean != u.path || !fs.ValidPath(clean) {
		return fmt.Errorf("alfred UI: %q should be a relative path ending in .alfredui", u.path)
	}
	if u.files == nil {
		return fmt.Errorf("alfred UI %s: no files", u.path)
	}
	if _, err := fs.Stat(u.files, "ui.json"); err != nil {
		return fmt.Errorf("alfred UI %s: ui.json: %w", u.path, err)
	}
	for name := range u.scripts {
		if name == "" || strings.ContainsAny(name, `/\`) || unsafeChars.MatchString(name) {
			return fmt.Errorf("alfred UI %s: script name %q may only contain letters, digits, '.', '_' and '-'", u.path, name)
		}
		if _, err := fs.Stat(u.files, "scripts/"+name); err == nil {
			return fmt.Errorf("alfred UI %s: scripts/%s is both a file and a Go script", u.path, name)
		}
	}
	return nil
}

func (u *UINode) object(string) (string, int, map[string]any) {
	mode := 0
	if u.external {
		mode = 1
	}
	return "alfred.workflow.userinterface.alfredui", 1, map[string]any{
		"path":       u.path,
		"stackview":  u.stack,
		"launchmode": mode,
	}
}

// packaged lists the package's files and a launcher for each Go script.
func (u *UINode) packaged(uid string) ([]packagedFile, error) {
	var files []packagedFile
	err := fs.WalkDir(u.files, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(u.files, name)
		if err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if info, err := d.Info(); err == nil && info.Mode()&0o111 != 0 || strings.HasPrefix(string(data), "#!") {
			mode = 0o755
		}
		files = append(files, packagedFile{name: path.Join(u.path, name), data: data, mode: mode})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("alfred UI %s: %w", u.path, err)
	}
	// Scripts run from anywhere, so each launcher finds the program relative
	// to itself: up out of scripts/ and the package's own directories.
	up := strings.Repeat("../", strings.Count(u.path, "/")+2)
	names := make([]string, 0, len(u.scripts))
	for name := range u.scripts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		script := fmt.Sprintf("#!/bin/sh\nexec \"$(dirname \"$0\")/%s%s\" ui %s %s \"$@\"\n", up, binaryName, uid, name)
		files = append(files, packagedFile{name: path.Join(u.path, "scripts", name), data: []byte(script), mode: 0o755})
	}
	return files, nil
}

func (u *UINode) runScript(name string, args []string, out io.Writer) error {
	handler, ok := u.scripts[name]
	if !ok {
		return fmt.Errorf("alfred UI %s has no script %q", u.path, name)
	}
	result, err := callScript(handler, args)
	if err != nil || result == nil {
		return err
	}
	return json.NewEncoder(out).Encode(result)
}

// callScript calls handler, turning a panic into an error.
func callScript(handler UIScript, args []string) (result any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	if handler == nil {
		return nil, errors.New("nil script")
	}
	return handler(args)
}
