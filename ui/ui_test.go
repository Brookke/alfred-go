package ui

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func decode(t *testing.T, v any) map[string]any {
	t.Helper()
	var b []byte
	var err error
	if doc, ok := v.(*Document); ok {
		b, err = doc.JSON("com.example.test")
	} else {
		b, err = json.Marshal(v)
	}
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s: %v", b, err)
	}
	return m
}

func get(m any, path ...any) any {
	for _, p := range path {
		switch k := p.(type) {
		case string:
			m = m.(map[string]any)[k]
		case int:
			m = m.([]any)[k]
		}
	}
	return m
}

type searchArgs struct {
	Query string
	Kind  string
	Limit int
	Exact bool
}

// notes is a small interface: a search field, kind picker, results list and status.
type notes struct {
	doc     *Document
	query   *TextFieldComponent
	kinds   *PickerComponent
	title   *LabelComponent
	icon    *ImageViewComponent
	results *DataViewComponent
	status  *LabelComponent
	search  *Script[searchArgs]
}

func newNotes() *notes {
	n := &notes{}
	n.query = TextField().ID("query").Placeholder("Search").Focus()
	n.kinds = Picker(Item("All", ""), Divider(), Item("Text", "text")).ID("kinds")
	n.title = Label().ID("title").LineBreak(WordWrap)
	n.icon = ImageView().ID("icon").Square(16)
	n.status = Label().ID("status")
	n.results = DataView(Cell(HStack(n.icon, n.title).ID("cell").Padding(4)).
		Height(30).
		Init(n.title.Patch().Colour(LabelColour)).
		Selected(n.title.Patch().Colour(SelectedText))).
		ID("results").Cols(2)
	n.search = NewScript("search", func(a searchArgs) (Actions, error) {
		if a.Query == "fail" {
			return Actions{}, errors.New("nope")
		}
		return Do(
			Update(
				n.results.Patch().Rows(
					Row("1", n.title.Patch().Text(a.Query+" "+a.Kind), n.icon.Patch().Image(Symbol("star"))).With("path", "/a"),
				),
				n.status.Patch().Text("1 result"),
			),
			Update(n.status.Patch().Colour(SecondaryLabel)),
		), nil
	})
	run := n.search.Run(searchArgs{Query: n.query.TextValue(), Kind: n.kinds.SelectedValue(), Limit: 5, Exact: true})
	n.doc = Window(
		Border(
			HStack(n.query.OnChanged(run), n.kinds.OnChanged(run)).Region(Top),
			n.results.Region(Centre).OnActioned(Finish(n.results.Selected("path")).Var("mode", "open")),
			HStack(n.status, Button("Clear").On(Actioned, Confirm("Clear?", "Clear", Do(Update(n.query.Patch().Text("")))))).Region(Bottom),
		).ID("root").
			Shortcut("↓", Call(n.results, SelectDown)).
			Shortcut("⌘↩", Finish(n.results.SelectedUID())),
	).Size(600, 400).Persist("kind", "").Arg("input")
	return n
}

func TestDocumentJSON(t *testing.T) {
	n := newNotes()
	m := decode(t, n.doc)

	if m["bundleid"] != "com.example.test" || get(m, "window", "size", "width") != 600.0 {
		t.Errorf("document = %v", m)
	}
	if get(m, "variablesconfig", "kind", "persist") != true || get(m, "variablesconfig", "input", "src") != "args[0]" {
		t.Errorf("variablesconfig = %v", m["variablesconfig"])
	}
	root := get(m, "window", "root")
	if get(root, "layout") != "border" || get(root, "id") != "root" {
		t.Errorf("root = %v", root)
	}
	top := get(root, "components", 0)
	if get(top, "constraints", "region") != "top" || get(top, "layout") != "inline" {
		t.Errorf("top = %v", top)
	}
	query := get(top, "components", 0)
	if get(query, "type") != "textfield" || get(query, "focus") != true {
		t.Errorf("query = %v", query)
	}
	action := get(query, "events", 0, "action")
	want := map[string]any{"type": "script", "src": "scripts/search", "args": []any{"{value:query.text}", "{value:kinds.selected.value}", "5", "true"}}
	if b, _ := json.Marshal(action); string(b) != string(must(json.Marshal(want))) {
		t.Errorf("search action = %s", b)
	}

	results := get(root, "components", 1)
	if get(results, "cols") != 2.0 || get(results, "cell", "height") != 30.0 || get(results, "components") != nil {
		t.Errorf("results = %v", results)
	}
	if get(results, "cell", "init", "title.colour") != "labelColor" || get(results, "cell", "selected", "title.colour") != "alfredSelectedResultTextColor" {
		t.Errorf("cell styles = %v", get(results, "cell"))
	}
	if get(results, "cell", "root", "components", 1, "linebreak") != "wordwrap" {
		t.Errorf("cell root = %v", get(results, "cell", "root"))
	}
	finish := get(results, "events", 0, "action", "src", "finish")
	if get(finish, "args") != "{value:results.selected.path}" || get(finish, "variables", "mode") != "open" {
		t.Errorf("finish = %v", finish)
	}

	clear := get(root, "components", 2, "components", 1, "events", 0)
	if get(clear, "confirm", "message") != "Clear?" || get(clear, "action", "src", "update", "ids", "query", "text") != "" {
		t.Errorf("confirmed event = %v", clear)
	}
	shortcuts := get(root, "shortcuts").([]any)
	if get(shortcuts[0], "action", "type") != "function" || get(shortcuts[0], "action", "args") != "selectdown" ||
		get(shortcuts[1], "action", "src", "finish", "args") != "{value:results.selected.uid}" {
		t.Errorf("shortcuts = %v", shortcuts)
	}
}

func must(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return b
}

func TestScripts(t *testing.T) {
	n := newNotes()
	scripts := n.doc.Scripts()
	if len(scripts) != 1 || scripts["search"] == nil {
		t.Fatalf("scripts = %v", scripts)
	}
	out, err := scripts["search"]([]string{"pears", "<null>", "3", "false"})
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, out)
	row := get(m, "update", "ids", "results", "data", "src", 0)
	if get(row, "uid") != "1" || get(row, "title.text") != "pears " || get(row, "icon.image", "src") != "star" || get(row, "path") != "/a" {
		t.Errorf("row = %v", row)
	}
	if get(m, "update", "ids", "status", "text") != "1 result" || get(m, "update", "ids", "status", "colour") != "secondaryLabelColor" {
		t.Errorf("two updates of one component should merge: %v", get(m, "update", "ids", "status"))
	}

	if _, err := scripts["search"]([]string{"fail"}); err == nil {
		t.Error("script error should be returned")
	}
	if _, err := scripts["search"]([]string{"x", "", "many"}); err == nil || !strings.Contains(err.Error(), "Limit") {
		t.Errorf("bad int = %v", err)
	}
	args, _ := n.search.Call(nil)
	if args.commands == nil {
		t.Error("missing args should be zero values")
	}

	empty := NewScript("noop", func(struct{}) (Actions, error) { return Actions{}, nil })
	if out, err := empty.call(nil); out != nil || err != nil {
		t.Errorf("empty actions = %v, %v", out, err)
	}
}

func TestPatches(t *testing.T) {
	l := Label().ID("l").Text("a")
	p := l.Patch().Text("b").Hidden(true)
	if l.props["text"] != "a" || p.Name() != "l" {
		t.Errorf("patching changed the component: %v", l.props)
	}
	m := decode(t, Do(Update(p)))
	if get(m, "update", "ids", "l", "text") != "b" || get(m, "update", "ids", "l", "hidden") != true {
		t.Errorf("update = %v", m)
	}
	if get(decode(t, Do(Update(ImageView().ID("i").Patch().Image(NoImage)))), "update", "ids", "i", "image") != nil {
		t.Error("NoImage should be null")
	}
	colour := decode(t, map[string]any{"c": Adaptive(Hex("#fff"), Hex("#000"))})
	if get(colour, "c", "dark") != "#000" {
		t.Errorf("adaptive = %v", colour)
	}
	sel := decode(t, Do(Update(DataView(Cell(Label())).ID("d").Patch().Select("x"))))
	if get(sel, "update", "ids", "d", "data", "select") != "x" || get(sel, "update", "ids", "d", "data", "src") != nil {
		t.Errorf("select alone shouldn't replace rows: %v", sel)
	}
	for name, f := range map[string]func(){
		"patch in update of component": func() { Do(Update(Label())) },
		"patch constraint":             func() { Label().Patch().Region(Top) },
		"patch event":                  func() { Label().Patch().OnChanged() },
		"non-struct script":            func() { NewScript("x", func(string) (Actions, error) { return Actions{}, nil }) },
		"float field":                  func() { NewScript("x", func(struct{ F float64 }) (Actions, error) { return Actions{}, nil }) },
		"placeholder before creation":  func() { var l *LabelComponent; _ = l.Get("text") },
		"call before creation":         func() { var d *DataViewComponent; Call(d, SelectUp) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s should panic", name)
				}
			}()
			f()
		}()
	}
}

func TestInvalidDocuments(t *testing.T) {
	a := NewScript("same", func(struct{}) (Actions, error) { return Actions{}, nil })
	b := NewScript("same", func(struct{}) (Actions, error) { return Actions{}, nil })
	l := Label()
	for name, doc := range map[string]*Document{
		"duplicate IDs":     Window(VStack(Label().ID("x"), Label().ID("x"))),
		"patch in layout":   Window(VStack(l.Patch())),
		"same script names": Window(VStack(Button("a").OnActioned(a.Run(struct{}{})), Button("b").OnActioned(b.Run(struct{}{})))),
		"no root":           Window(nil),
	} {
		if _, err := doc.JSON(""); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestGeneratedIDs(t *testing.T) {
	a, b := Label(), Label()
	if a.Name() == b.Name() || !strings.HasPrefix(a.Name(), "label") {
		t.Errorf("ids = %s, %s", a.Name(), b.Name())
	}
}
