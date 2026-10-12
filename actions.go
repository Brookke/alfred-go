package alfred

import (
	"fmt"
	"io"
	"maps"
	"strings"
)

// ActionNode is an Alfred action or output.
type ActionNode struct {
	node[*ActionNode]
	kind    string
	version int
	config  map[string]any
	handler func(arg string) (string, error)
}

func newAction(label, kind string, version int, config map[string]any) *ActionNode {
	a := &ActionNode{kind: kind, version: version, config: config}
	a.self, a.label = a, label
	return a
}

// OpenURL opens its input, which should be a URL, in the default browser.
func OpenURL() *ActionNode {
	return newAction("openurl", "alfred.workflow.action.openurl", 1, map[string]any{
		"url":     "{query}",
		"browser": "",
		"spaces":  "",
		"utf8":    true,
	})
}

// Copy copies its input to the clipboard.
func Copy() *ActionNode {
	return newAction("copy", "alfred.workflow.output.clipboard", 3, map[string]any{
		"clipboardtext":             "{query}",
		"autopaste":                 false,
		"transient":                 false,
		"ignoredynamicplaceholders": false,
	})
}

// Paste copies its input and pastes it into the frontmost app.
func Paste() *ActionNode {
	return Copy().Set("autopaste", true).relabel("paste")
}

// Notify posts a macOS notification with title, and its input as the text.
func Notify(title string) *ActionNode {
	return newAction("notify", "alfred.workflow.output.notification", 1, map[string]any{
		"title":                    title,
		"text":                     "{query}",
		"onlyshowifquerypopulated": true,
		"lastpathcomponent":        false,
		"removeextension":          false,
	})
}

// Run calls handler with its input. Whatever handler returns is passed on to
// the nodes connected with Then.
func Run(handler func(arg string) (string, error)) *ActionNode {
	a := newAction("run", "alfred.workflow.action.script", 2, map[string]any{
		"scriptargtype": 1,
		"scriptfile":    "",
		"type":          0,
		"escaping":      102,
		"concurrently":  false,
	})
	a.handler = handler
	return a
}

// Raw adds any other Alfred object, as it appears in an exported info.plist,
// such as "alfred.workflow.output.largetype".
func Raw(kind string, version int, config map[string]any) *ActionNode {
	return newAction(kind[strings.LastIndex(kind, ".")+1:], kind, version, maps.Clone(config))
}

// Set overrides one key of the object's Alfred config, for options this
// package doesn't model.
func (a *ActionNode) Set(key string, value any) *ActionNode {
	a.config = maps.Clone(a.config)
	if a.config == nil {
		a.config = make(map[string]any)
	}
	a.config[key] = value
	return a
}

func (a *ActionNode) relabel(label string) *ActionNode {
	a.label = label
	return a
}

func (a *ActionNode) object(uid string) (string, int, map[string]any) {
	config := maps.Clone(a.config)
	if config == nil {
		config = make(map[string]any)
	}
	if a.handler != nil {
		config["script"] = dispatch(uid)
	}
	return a.kind, a.version, config
}

func (a *ActionNode) run(arg string, out io.Writer) error {
	if a.handler == nil {
		return fmt.Errorf("%s has no Go handler", a.kind)
	}
	result, err := a.handler(arg)
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, result)
	return err
}
