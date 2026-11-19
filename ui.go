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

	"github.com/brookke/alfred-go/ui"
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
	doc             *ui.Document
	external, stack bool
	scripts         map[string]UIScript
	// debugLog, set by the dev command, logs everything to this file.
	debugLog string
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

// UI shows the interface doc, declared in Go with package ui, packaged into
// the workflow as name, such as "notes.alfredui". The scripts its actions
// run are packaged with it. Nodes connected with Then receive the args of
// ui.Finish.
func UI(name string, doc *ui.Document) *UINode {
	u := AlfredUI(name, nil)
	u.doc = doc
	return u
}

// Files adds files, such as images, to a UI declared with UI. Paths in the
// document are relative to the package.
func (u *UINode) Files(files fs.FS) *UINode {
	u.files = files
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
	if u.doc != nil {
		if u.files != nil {
			if _, err := fs.Stat(u.files, "ui.json"); err == nil {
				return fmt.Errorf("alfred UI %s: has both a document and a ui.json file", u.path)
			}
		}
		if _, err := u.doc.JSON(""); err != nil {
			return fmt.Errorf("alfred UI %s: %w", u.path, err)
		}
	} else if u.files == nil {
		return fmt.Errorf("alfred UI %s: no files", u.path)
	} else if _, err := fs.Stat(u.files, "ui.json"); err != nil {
		return fmt.Errorf("alfred UI %s: ui.json: %w", u.path, err)
	}
	for name := range u.allScripts() {
		if name == "" || strings.ContainsAny(name, `/\`) || unsafeChars.MatchString(name) {
			return fmt.Errorf("alfred UI %s: script name %q may only contain letters, digits, '.', '_' and '-'", u.path, name)
		}
		if _, exists := u.scripts[name]; exists && u.doc != nil && u.doc.Scripts()[name] != nil {
			return fmt.Errorf("alfred UI %s: two scripts are called %q", u.path, name)
		}
		if u.files == nil {
			continue
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

// allScripts are the Go scripts the UI runs: those added with Script and
// those its document uses.
func (u *UINode) allScripts() map[string]UIScript {
	all := map[string]UIScript{}
	if u.doc != nil {
		for name, fn := range u.doc.Scripts() {
			all[name] = fn
		}
	}
	for name, fn := range u.scripts {
		all[name] = fn
	}
	return all
}

// packaged lists the package's files and a launcher for each Go script.
func (u *UINode) packaged(uid, bundleID string) ([]packagedFile, error) {
	var files []packagedFile
	if u.doc != nil {
		data, err := u.doc.JSON(bundleID)
		if err == nil && u.debugLog != "" {
			data, err = withDebugLog(data, u.debugLog)
		}
		if err != nil {
			return nil, fmt.Errorf("alfred UI %s: %w", u.path, err)
		}
		files = append(files, packagedFile{name: path.Join(u.path, "ui.json"), data: data, mode: 0o644})
	}
	if u.files == nil {
		return u.withLaunchers(uid, files), nil
	}
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
	if u.debugLog != "" && u.doc == nil {
		for i, f := range files {
			if f.name == path.Join(u.path, "ui.json") {
				if files[i].data, err = withDebugLog(f.data, u.debugLog); err != nil {
					return nil, fmt.Errorf("alfred UI %s: %w", u.path, err)
				}
			}
		}
	}
	return u.withLaunchers(uid, files), nil
}

// withLaunchers adds a launcher for each Go script.
func (u *UINode) withLaunchers(uid string, files []packagedFile) []packagedFile {
	scripts := u.allScripts()
	// Scripts run from anywhere, so each launcher finds the program relative
	// to itself: up out of scripts/ and the package's own directories.
	up := strings.Repeat("../", strings.Count(u.path, "/")+2)
	names := make([]string, 0, len(scripts))
	for name := range scripts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		script := fmt.Sprintf("#!/bin/sh\nexec \"$(dirname \"$0\")/%s%s\" ui %s %s \"$@\"\n", up, binaryName, uid, name)
		files = append(files, packagedFile{name: path.Join(u.path, "scripts", name), data: []byte(script), mode: 0o755})
	}
	return files
}

// withDebugLog makes a ui.json log everything to path.
func withDebugLog(data []byte, path string) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("ui.json: %w", err)
	}
	m["log"] = map[string]any{"path": path, "level": "all", "timestamp": true}
	return json.MarshalIndent(m, "", "  ")
}

func (u *UINode) runScript(name string, args []string, out io.Writer) error {
	handler, ok := u.allScripts()[name]
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
