package ui

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
)

// Document is a whole interface: a window and its settings.
type Document struct {
	root    Component
	window  map[string]any
	top     map[string]any
	vars    map[string]any
	config  map[string]any
	colours map[string]any
	errors  *LabelComponent
}

// Window is an interface whose window shows root.
func Window(root Component) *Document {
	return &Document{root: root, window: map[string]any{}, top: map[string]any{}}
}

// Size is the window's size. Inside Alfred, Alfred may adjust it.
func (d *Document) Size(width, height int) *Document {
	d.window["size"] = map[string]any{"width": width, "height": height}
	return d
}

func (d *Document) Padding(padding int) *Document {
	d.window["padding"] = padding
	return d
}

// Title is the window's title when it opens outside Alfred.
func (d *Document) Title(title string) *Document {
	d.window["title"] = title
	return d
}

// SetWindow sets any window property, as Alfred's reference names it.
func (d *Document) SetWindow(key string, value any) *Document {
	d.window[key] = value
	return d
}

// Set sets any top-level ui.json property, such as "pulse".
func (d *Document) Set(key string, value any) *Document {
	d.top[key] = value
	return d
}

// Log sets the log level: "error", "warning", "info", "debug" or "all".
// Logs go to the workflow debugger in Alfred Preferences.
func (d *Document) Log(level string) *Document {
	d.top["log"] = map[string]any{"level": level, "internal": true}
	return d
}

// Var sets an interface variable's initial value, read with Var.
func (d *Document) Var(name, value string) *Document {
	if d.vars == nil {
		d.vars = map[string]any{}
	}
	d.vars[name] = value
	return d
}

// Persist keeps a variable's value between launches, set with SetVar.
func (d *Document) Persist(name, defaultValue string) *Document {
	d.varConfig(name)["default"] = defaultValue
	d.varConfig(name)["persist"] = true
	return d
}

// Arg sets a variable to the input the interface was opened with.
func (d *Document) Arg(name string) *Document {
	d.varConfig(name)["src"] = "args[0]"
	return d
}

// WorkflowVar makes a workflow variable available to the interface.
func (d *Document) WorkflowVar(name string) *Document {
	d.varConfig(name)["src"] = "variable"
	return d
}

func (d *Document) varConfig(name string) map[string]any {
	if d.config == nil {
		d.config = map[string]any{}
	}
	c, ok := d.config[name].(map[string]any)
	if !ok {
		c = map[string]any{}
		d.config[name] = c
	}
	return c
}

// Colour names a colour, for use as System(name).
func (d *Document) Colour(name string, colour Colour) *Document {
	if d.colours == nil {
		d.colours = map[string]any{}
	}
	d.colours[name] = colour
	return d
}

// walk visits every component, including cell roots.
func walk(c Component, visit func(*core)) {
	b := c.base()
	visit(b)
	for _, child := range b.children {
		walk(child, visit)
	}
	if b.cell != nil {
		walk(b.cell, visit)
	}
}

// JSON is the document's ui.json. bundleID identifies the workflow, which
// Alfred needs to persist variables.
func (d *Document) JSON(bundleID string) ([]byte, error) {
	if d.root == nil {
		return nil, fmt.Errorf("ui: the window has no root component")
	}
	return d.json(bundleID)
}

func (d *Document) json(bundleID string) (data []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	ids := map[string]bool{}
	walk(d.root, func(c *core) {
		if c.target != nil && err == nil {
			err = fmt.Errorf("ui: a patch of %s is in the layout; add the component itself", c.target.id)
		}
		if c.kind == "dataview" && c.target == nil && c.cell == nil && err == nil {
			err = fmt.Errorf("ui: data view %s has no Cell", c.id)
		}
		if ids[c.id] && err == nil {
			err = fmt.Errorf("ui: two components have ID %q", c.id)
		}
		ids[c.id] = true
	})
	if err != nil {
		return nil, err
	}
	if _, err := d.scripts(); err != nil {
		return nil, err
	}
	m := maps.Clone(d.top)
	if bundleID != "" {
		m["bundleid"] = bundleID
	}
	window := maps.Clone(d.window)
	window["root"] = d.root.base().json()
	m["window"] = window
	if d.vars != nil {
		m["variables"] = d.vars
	}
	if d.config != nil {
		m["variablesconfig"] = d.config
	}
	if d.colours != nil {
		m["colours"] = d.colours
	}
	data, err = json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return resolve(data)
}

func (d *Document) scripts() (map[string]runner, error) {
	scripts := map[string]runner{}
	var err error
	if d.root == nil {
		return scripts, nil
	}
	walk(d.root, func(c *core) {
		var actions []Action
		for _, e := range c.events {
			actions = append(actions, e.actions...)
		}
		for _, sc := range c.shortcuts {
			actions = append(actions, sc.actions...)
		}
		for _, a := range actions {
			for _, s := range a.uses() {
				if other, ok := scripts[s.scriptName()]; ok && other != s && err == nil {
					err = fmt.Errorf("ui: two scripts are called %q", s.scriptName())
				}
				scripts[s.scriptName()] = s
			}
		}
	})
	return scripts, err
}

// ErrorLabel shows the errors scripts return in label, in red. Without one,
// a failing script only writes its error to stderr, which Alfred logs.
func (d *Document) ErrorLabel(label *LabelComponent) *Document {
	d.errors = label
	return d
}

// Scripts are the scripts the document's actions run, by name, for
// alfred.UI to package and call.
func (d *Document) Scripts() map[string]func(args []string) (any, error) {
	scripts, _ := d.scripts()
	out := make(map[string]func([]string) (any, error), len(scripts))
	for name, s := range scripts {
		out[name] = d.reporting(s.call)
	}
	return out
}

// reporting shows a script's error in the error label, if there is one.
func (d *Document) reporting(call func([]string) (any, error)) func([]string) (any, error) {
	if d.errors == nil {
		return call
	}
	return func(args []string) (result any, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("%v", r)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				result, err = Apply(Update(d.errors.Patch().Text(err.Error()).Colour(System("systemRedColor")))), nil
			}
		}()
		return call(args)
	}
}
