package alfred

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// bundleID is the running workflow's, for DataDir and CacheDir outside Alfred.
var bundleID string

// DataDir is the folder for the workflow's own data, such as settings and
// saved items. Alfred passes it to handlers; outside Alfred, such as with go
// run . run, it's where Alfred would put it. It may not exist yet.
func DataDir() string {
	return alfredDir("alfred_workflow_data", "Library/Application Support/Alfred/Workflow Data")
}

// CacheDir is the folder for data the workflow can recreate, such as
// thumbnails. It may not exist yet.
func CacheDir() string {
	return alfredDir("alfred_workflow_cache", "Library/Caches/com.runningwithcrayons.Alfred/Workflow Data")
}

func alfredDir(env, fallback string) string {
	if dir := os.Getenv(env); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, fallback, bundleID)
}

// alfredPreferences is the folder of the user's Alfred preferences, from
// Alfred's prefs.json.
func alfredPreferences() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(home, "Library/Application Support/Alfred/prefs.json"))
	if err != nil {
		return "", fmt.Errorf("finding Alfred's preferences: %w", err)
	}
	var prefs struct{ Current string }
	if err := json.Unmarshal(data, &prefs); err != nil || prefs.Current == "" {
		return "", errors.New("finding Alfred's preferences: prefs.json has no current preferences")
	}
	return prefs.Current, nil
}

// installedFolder is the folder in workflows holding the workflow with
// bundleID, or a new folder for it.
func installedFolder(workflows, bundleID string) (string, error) {
	entries, err := os.ReadDir(workflows)
	if err != nil {
		return "", err
	}
	needle := []byte("<string>" + bundleID + "</string>")
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		plist, err := os.ReadFile(filepath.Join(workflows, e.Name(), "info.plist"))
		if err != nil {
			continue
		}
		// The bundle ID is the string after the bundleid key, wherever it is.
		if i := bytes.Index(plist, []byte("<key>bundleid</key>")); i >= 0 && bytes.HasPrefix(bytes.TrimLeft(plist[i+len("<key>bundleid</key>"):], " \t\r\n"), needle) {
			return filepath.Join(workflows, e.Name()), nil
		}
	}
	return filepath.Join(workflows, "user.workflow."+unsafeChars.ReplaceAllString(bundleID, "-")), nil
}

// dev builds the workflow, installs it straight into Alfred's preferences,
// replacing any copy with the same bundle ID, and has Alfred reload it.
// With logs, Alfred UIs log everything to <bundle id>.log in Alfred's UI
// logs folder, whose path it prints.
func (w *WorkflowView) dev(logs bool, stdout io.Writer) error {
	if w.bundleID == "" {
		return errors.New("workflow needs a BundleID")
	}
	prefs, err := alfredPreferences()
	if err != nil {
		return err
	}
	logName := ""
	if logs {
		logName = unsafeChars.ReplaceAllString(w.bundleID, "-") + ".log"
	}
	g, err := w.graph()
	if err != nil {
		return err
	}
	for _, n := range g.nodes {
		if u, ok := n.(*UINode); ok {
			u.debugLog = logName
			defer func() { u.debugLog = "" }()
		}
	}
	tmp, err := os.MkdirTemp("", "alfred-dev")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	built, err := w.build(tmp)
	if err != nil {
		return err
	}
	folder, err := installedFolder(filepath.Join(prefs, "workflows"), w.bundleID)
	if err != nil {
		return err
	}
	if err := unpack(built, folder); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "Installed", folder)
	script := fmt.Sprintf(`tell application id "com.runningwithcrayons.Alfred" to reload workflow %q`, w.bundleID)
	if out, err := exec.Command("osascript", "-e", script).CombinedOutput(); err != nil {
		fmt.Fprintf(stdout, "Reload the workflow in Alfred Preferences: %s\n", strings.TrimSpace(string(out)))
	}
	if logs {
		home, _ := os.UserHomeDir()
		fmt.Fprintln(stdout, "Alfred UI log:", filepath.Join(home, "Library/Application Support/Alfred/AlfredUI/Logs", logName))
	}
	return nil
}

// unpack replaces folder's contents with the workflow archive's.
func unpack(archive, folder string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := os.RemoveAll(folder); err != nil {
		return err
	}
	for _, f := range r.File {
		path := filepath.Join(folder, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(path, folder+string(filepath.Separator)) {
			return fmt.Errorf("unpacking: bad path %q", f.Name)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		src, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(src)
		src.Close()
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, data, f.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}
