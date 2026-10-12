package alfred

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func testWorkflow() *WorkflowView {
	open := OpenURL()
	shout := Run(func(arg string) (string, error) { return strings.ToUpper(arg), nil })
	return Workflow("Test PRs",
		ScriptFilter("pr", func(query string) Output {
			if query == "panic" {
				panic("boom")
			}
			return Items{Item("PR " + query).Arg("https://example.com/" + query)}
		}).NoSpace().Title("Open PR").Then(open).On(Cmd|Shift, "Copy URL", Copy()),
		Keyword("go").Then(open, shout.Then(Notify("Done"))),
	).BundleID("com.example.test").Version("1.0.0")
}

func runWorkflow(w *WorkflowView, args ...string) (string, int) {
	var out, errs bytes.Buffer
	code := w.command(append([]string{"run"}, args...), &out, &errs)
	return out.String() + errs.String(), code
}

func marshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGraphSharesNodesAndConnectsModifiers(t *testing.T) {
	p, err := testWorkflow().plist()
	if err != nil {
		t.Fatal(err)
	}
	var uids []string
	objects := p["objects"].([]any)
	for _, o := range objects {
		uids = append(uids, o.(map[string]any)["uid"].(string))
	}
	want := []string{"pr", "pr.openurl", "pr.copy", "go", "go.run", "go.run.notify"}
	if !slices.Equal(uids, want) {
		t.Fatalf("uids = %v, want %v", uids, want)
	}

	connections := p["connections"].(map[string]any)
	copyLink := connections["pr"].([]any)[1].(map[string]any)
	if copyLink["destinationuid"] != "pr.copy" || copyLink["modifiers"] != int(Cmd|Shift) || copyLink["modifiersubtext"] != "Copy URL" {
		t.Errorf("modifier connection = %v", copyLink)
	}
	if shared := connections["go"].([]any)[0].(map[string]any); shared["destinationuid"] != "pr.openurl" {
		t.Errorf("go should reuse the shared OpenURL, got %v", shared)
	}

	config := objects[0].(map[string]any)["config"].(map[string]any)
	if config["script"] != `exec ./workflow run pr "$1"` || config["withspace"] != false {
		t.Errorf("script filter config = %v", config)
	}
	if _, ok := objects[4].(map[string]any)["config"].(map[string]any)["script"]; !ok {
		t.Error("Run action should dispatch back to the program")
	}
}

func TestRunCallsHandlers(t *testing.T) {
	w := testWorkflow()
	var got struct {
		Items []struct{ Title, Subtitle, Arg string }
	}

	out, code := runWorkflow(w, "pr", "42")
	if err := json.Unmarshal([]byte(out), &got); err != nil || code != 0 {
		t.Fatalf("run pr = %q, %d, %v", out, code, err)
	}
	if got.Items[0].Title != "PR 42" || got.Items[0].Arg != "https://example.com/42" {
		t.Errorf("items = %+v", got.Items)
	}

	if out, _ := runWorkflow(w, "go.run", "hello"); out != "HELLO" {
		t.Errorf("run action = %q", out)
	}

	out, code = runWorkflow(w, "pr", "panic")
	if err := json.Unmarshal([]byte(out), &got); err != nil || code != 0 || !strings.Contains(got.Items[0].Subtitle, "boom") {
		t.Errorf("panicking filter = %q, %d", out, code)
	}

	if _, code := runWorkflow(w, "missing"); code != 1 {
		t.Errorf("unknown uid exit code = %d", code)
	}
}

func TestItemsAreValues(t *testing.T) {
	base := Item("PR").Arg("a")
	varied := base.Mod(Cmd|Alt, "Copy", "b").Copy("c").Invalid()

	if got := marshal(t, base); got != `{"title":"PR","arg":"a"}` {
		t.Errorf("base changed: %s", got)
	}
	got := marshal(t, varied)
	for _, want := range []string{`"cmd+alt":{"valid":true,"arg":"b","subtitle":"Copy"}`, `"copy":"c"`, `"valid":false`} {
		if !strings.Contains(got, want) {
			t.Errorf("%s does not contain %s", got, want)
		}
	}

	response := Response{
		Items: Items{base},
		Rerun: 500 * time.Millisecond,
		Vars: map[string]string{
			"k": "v",
		},
	}
	if got := marshal(t, response); got != `{"items":[{"title":"PR","arg":"a"}],"rerun":0.5,"variables":{"k":"v"}}` {
		t.Errorf("response = %s", got)
	}
	if got := marshal(t, Response{}); got != `{"items":[]}` {
		t.Errorf("empty response = %s", got)
	}
}

func TestInvalidWorkflows(t *testing.T) {
	for name, w := range map[string]*WorkflowView{
		"no bundle ID": Workflow("x", Keyword("a")),
		"unsafe ID":    Workflow("x", Keyword("a").ID("bad id")).BundleID("b"),
		"duplicate ID": Workflow("x", Keyword("a").ID("same"), Keyword("b").ID("same")).BundleID("b"),
		"nil node":     Workflow("x", Keyword("a").Then(nil)).BundleID("b"),
	} {
		if _, err := w.plist(); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestPlistIsValid(t *testing.T) {
	plutil, err := exec.LookPath("plutil")
	if err != nil {
		t.Skip("plutil is only on macOS")
	}
	readme := "Line one\n<b>Tom & Jerry</b>"
	p, err := testWorkflow().Readme(readme).Var("token", "x").plist()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "info.plist")
	var b bytes.Buffer
	if err := encodePlist(&b, p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(plutil, "-lint", path).CombinedOutput(); err != nil {
		t.Fatalf("plutil -lint: %s", out)
	}
	out, err := exec.Command(plutil, "-extract", "readme", "raw", "-o", "-", path).Output()
	if err != nil || strings.TrimSuffix(string(out), "\n") != readme {
		t.Errorf("readme round trip = %q, %v", out, err)
	}
}

func TestBuildPackagesExecutable(t *testing.T) {
	path, err := testWorkflow().build(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "Test-PRs.alfredworkflow" {
		t.Errorf("path = %s", path)
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	modes := make(map[string]os.FileMode)
	for _, f := range archive.File {
		modes[f.Name] = f.Mode()
	}
	if _, ok := modes["info.plist"]; !ok {
		t.Error("missing info.plist")
	}
	if modes["workflow"]&0o111 == 0 {
		t.Errorf("workflow binary mode = %v, want executable", modes["workflow"])
	}
}
