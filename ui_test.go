package alfred

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/brookke/alfred-go/ui"
)

func uiFiles() fstest.MapFS {
	return fstest.MapFS{
		"ui.json":            {Data: []byte(`{"window":{"root":{"type":"view"}}}`)},
		"components/a.json":  {Data: []byte(`{}`)},
		"scripts/hello.zsh":  {Data: []byte("#!/bin/zsh\necho '{}'\n")},
		"resources/logo.png": {Data: []byte("png"), Mode: 0o644},
	}
}

func uiWorkflow() *WorkflowView {
	ui := AlfredUI("picker.alfredui", uiFiles()).
		Script("list", func(args []string) (any, error) {
			return UIActions{"update": map[string]any{"ids": map[string]any{
				"label": map[string]any{"text": strings.Join(args, ",")},
			}}}, nil
		}).
		Script("quiet", func([]string) (any, error) { return nil, nil }).
		Script("fail", func([]string) (any, error) { return nil, errors.New("nope") }).
		Script("panic", func([]string) (any, error) { panic("boom") }).
		Then(Paste())
	return Workflow("UI", Keyword("pick").Then(ui), External("open").Then(ui)).BundleID("com.example.ui")
}

func TestAlfredUIObject(t *testing.T) {
	p, err := uiWorkflow().plist()
	if err != nil {
		t.Fatal(err)
	}
	objects := p["objects"].([]any)
	ui := objects[1].(map[string]any)
	if ui["type"] != "alfred.workflow.userinterface.alfredui" || ui["uid"] != "pick.picker" || ui["version"] != 1 {
		t.Errorf("ui object = %v", ui)
	}
	want := map[string]any{"path": "picker.alfredui", "stackview": false, "launchmode": 0}
	if got := marshal(t, ui["config"]); got != marshal(t, want) {
		t.Errorf("ui config = %s", got)
	}
	_, _, config := AlfredUI("x.alfredui", uiFiles()).External().Stack().object("")
	if config["launchmode"] != 1 || config["stackview"] != true {
		t.Errorf("external stacked config = %v", config)
	}
	trigger := objects[3].(map[string]any)
	if trigger["type"] != "alfred.workflow.trigger.external" || marshal(t, trigger["config"]) != `{"availableviaurlhandler":false,"triggerid":"open"}` {
		t.Errorf("external trigger = %v", trigger)
	}
	if links := p["connections"].(map[string]any)["open"].([]any); links[0].(map[string]any)["destinationuid"] != "pick.picker" {
		t.Errorf("external trigger should share the UI, got %v", links)
	}
}

func TestAlfredUIScripts(t *testing.T) {
	w := uiWorkflow()
	run := func(args ...string) (string, string, int) {
		var out, errs bytes.Buffer
		code := w.command(append([]string{"ui", "pick.picker"}, args...), &out, &errs)
		return out.String(), errs.String(), code
	}

	out, _, code := run("list", "a", "b c")
	var got map[string]map[string]map[string]map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil || code != 0 || got["update"]["ids"]["label"]["text"] != "a,b c" {
		t.Errorf("list = %q, %d, %v", out, code, err)
	}
	if out, _, code := run("quiet"); out != "" || code != 0 {
		t.Errorf("nil result = %q, %d", out, code)
	}
	for _, script := range []string{"fail", "panic", "missing"} {
		if _, errs, code := run(script); code != 1 || errs == "" {
			t.Errorf("%s = %q, %d", script, errs, code)
		}
	}
	var errs bytes.Buffer
	if code := w.command([]string{"ui", "pick"}, io.Discard, &errs); code != 1 {
		t.Errorf("ui on a keyword = %d", code)
	}
}

func TestAlfredUIPackaged(t *testing.T) {
	path, err := uiWorkflow().build(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	files := make(map[string]*zip.File)
	for _, f := range archive.File {
		files[f.Name] = f
	}
	for name, exec := range map[string]bool{
		"picker.alfredui/ui.json":            false,
		"picker.alfredui/components/a.json":  false,
		"picker.alfredui/resources/logo.png": false,
		"picker.alfredui/scripts/hello.zsh":  true,
		"picker.alfredui/scripts/list":       true,
		"picker.alfredui/scripts/panic":      true,
	} {
		f, ok := files[name]
		if !ok {
			t.Errorf("missing %s", name)
			continue
		}
		if got := f.Mode()&0o111 != 0; got != exec {
			t.Errorf("%s mode = %v", name, f.Mode())
		}
	}
	r, err := files["picker.alfredui/scripts/list"].Open()
	if err != nil {
		t.Fatal(err)
	}
	launcher, _ := io.ReadAll(r)
	if want := `exec "$(dirname "$0")/../../workflow" ui pick.picker list "$@"`; !strings.Contains(string(launcher), want) {
		t.Errorf("launcher = %q", launcher)
	}

	nested := AlfredUI("ui/deep.alfredui", uiFiles()).Script("s", nil)
	extra, err := nested.packaged("x", "b")
	if err != nil {
		t.Fatal(err)
	}
	if last := string(extra[len(extra)-1].data); !strings.Contains(last, `/../../../workflow" ui x s`) {
		t.Errorf("nested launcher = %q", last)
	}
}

func TestInvalidAlfredUI(t *testing.T) {
	clash := uiFiles()
	clash["scripts/list"] = &fstest.MapFile{Data: []byte("#!/bin/sh\n")}
	for name, ui := range map[string]*UINode{
		"no suffix":    AlfredUI("picker", uiFiles()),
		"absolute":     AlfredUI("/tmp/x.alfredui", uiFiles()),
		"parent":       AlfredUI("../x.alfredui", uiFiles()),
		"no ui.json":   AlfredUI("x.alfredui", fstest.MapFS{"a.json": {}}),
		"no files":     AlfredUI("x.alfredui", nil),
		"bad script":   AlfredUI("x.alfredui", uiFiles()).Script("a/b", nil),
		"script clash": AlfredUI("x.alfredui", clash).Script("list", nil),
	} {
		if _, err := Workflow("x", Keyword("a").Then(ui)).BundleID("b").plist(); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := Workflow("x", Keyword("a").Then(AlfredUI("x.alfredui", os.DirFS(t.TempDir())))).BundleID("b").plist(); err == nil {
		t.Error("empty directory: expected an error")
	}
}

func TestDeclaredUI(t *testing.T) {
	status := ui.Label().ID("status")
	hello := ui.NewScript("hello", func(a struct{ Name string }) (ui.Changes, error) {
		return ui.Apply(ui.Update(status.Patch().Text("Hello " + a.Name))), nil
	})
	name := ui.TextField().ID("name")
	doc := ui.Window(ui.VStack(name.OnChanged(hello.Run(struct{ Name string }{name.TextValue()})), status))
	w := Workflow("Hi", Keyword("hi").Then(UI("hi.alfredui", doc).Files(fstest.MapFS{"logo.png": {Data: []byte("png")}}))).BundleID("com.example.hi")

	var out, errs bytes.Buffer
	if code := w.command([]string{"ui", "hi.hi", "hello", "Ada"}, &out, &errs); code != 0 || !strings.Contains(out.String(), `"text":"Hello Ada"`) {
		t.Errorf("hello = %q, %q, %d", out.String(), errs.String(), code)
	}

	path, err := w.build(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	files := map[string]*zip.File{}
	for _, f := range archive.File {
		files[f.Name] = f
	}
	for _, name := range []string{"hi.alfredui/ui.json", "hi.alfredui/logo.png", "hi.alfredui/scripts/hello"} {
		if files[name] == nil {
			t.Errorf("missing %s", name)
		}
	}
	r, _ := files["hi.alfredui/ui.json"].Open()
	uiJSON, _ := io.ReadAll(r)
	if !strings.Contains(string(uiJSON), `"bundleid": "com.example.hi"`) || !strings.Contains(string(uiJSON), `"src": "scripts/hello"`) {
		t.Errorf("ui.json = %s", uiJSON)
	}

	clash := fstest.MapFS{"ui.json": {Data: []byte("{}")}}
	if _, err := Workflow("x", Keyword("a").Then(UI("x.alfredui", doc).Files(clash))).BundleID("b").plist(); err == nil {
		t.Error("a document and a ui.json file should be an error")
	}
	bad := ui.Window(ui.VStack(ui.Label().ID("same"), ui.Label().ID("same")))
	if _, err := Workflow("x", Keyword("a").Then(UI("x.alfredui", bad))).BundleID("b").plist(); err == nil {
		t.Error("an invalid document should be an error")
	}
}

func TestDevHelpers(t *testing.T) {
	t.Setenv("alfred_workflow_data", "")
	t.Setenv("alfred_workflow_cache", "/tmp/cache")
	bundleID = "com.example.dirs"
	if !strings.HasSuffix(DataDir(), "Alfred/Workflow Data/com.example.dirs") || CacheDir() != "/tmp/cache" {
		t.Errorf("dirs = %s, %s", DataDir(), CacheDir())
	}

	workflows := t.TempDir()
	os.MkdirAll(filepath.Join(workflows, "user.workflow.ABC"), 0o755)
	os.WriteFile(filepath.Join(workflows, "user.workflow.ABC", "info.plist"),
		[]byte("<dict>\n\t<key>bundleid</key>\n\t<string>com.example.hi</string>\n</dict>"), 0o644)
	if got, _ := installedFolder(workflows, "com.example.hi"); filepath.Base(got) != "user.workflow.ABC" {
		t.Errorf("existing install = %s", got)
	}
	if got, _ := installedFolder(workflows, "com.example.new"); filepath.Base(got) != "user.workflow.com.example.new" {
		t.Errorf("new install = %s", got)
	}

	built, err := uiWorkflow().build(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(workflows, "user.workflow.ABC")
	os.WriteFile(filepath.Join(folder, "stale"), nil, 0o644)
	if err := unpack(built, folder); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(folder, "stale")); !os.IsNotExist(err) {
		t.Error("unpack should replace the folder")
	}
	if info, err := os.Stat(filepath.Join(folder, "picker.alfredui/scripts/list")); err != nil || info.Mode()&0o111 == 0 {
		t.Errorf("launcher = %v, %v", info, err)
	}

	logged, err := withDebugLog([]byte(`{"window":{}}`), "x.log")
	if err != nil || !strings.Contains(string(logged), `"path": "x.log"`) {
		t.Errorf("debug log = %s, %v", logged, err)
	}
}
