// Package ui declares Alfred UI interfaces in Go.
//
// Alfred UI (Alfred 6) builds native interfaces from a ui.json file. This
// package builds that file from Go values instead, so components, the data
// shown in them and the scripts behind them share one set of names, checked
// by the compiler:
//
//	query := ui.TextField().Placeholder("Search")
//	status := ui.Label()
//	search := ui.NewScript("search", func(a struct{ Query string }) (ui.Actions, error) {
//		return ui.Do(ui.Update(status.Patch().Text("Searching for " + a.Query))), nil
//	})
//	doc := ui.Window(ui.VStack(
//		query.OnChanged(search.Run(struct{ Query string }{query.Get("text")})),
//		status,
//	))
//
// Pass the document to alfred.UI to add it to a workflow.
//
// Give an ID to every component that something refers to: a placeholder
// such as TextValue, a patch, or Call. Other components get an ID from
// their type and creation order, such as "label2". References resolve when
// the document is written, so a component can be named and configured after
// actions refer to it; it only has to exist. Anything this package
// doesn't model can be set with Set, using the keys in Alfred's reference:
// https://www.alfredapp.com/help/alfredui/.
package ui

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"strconv"
	"sync"
)

// Component is anything in an interface, such as a Label or a View.
type Component interface {
	base() *core
}

var (
	idMu     sync.Mutex
	idCounts = map[string]int{}
)

func nextID(kind string) string {
	idMu.Lock()
	defer idMu.Unlock()
	idCounts[kind]++
	return fmt.Sprintf("%s%d", kind, idCounts[kind])
}

// core is what every component has.
type core struct {
	kind        string
	id          string
	named       bool
	props       map[string]any
	constraints map[string]any
	events      []event
	children    []Component
	shortcuts   []shortcut
	// ref is the component's number in refs, plus one; 0 if never referred to.
	ref int
	// cell is a DataView's cell root, which isn't one of its components.
	cell Component
	// target is the component a patch changes; nil for real components.
	target *core
}

type event struct {
	trigger Trigger
	actions []Action
	confirm map[string]any
}

type shortcut struct {
	key     string
	actions []Action
}

// name is the ID of a component something refers to, which must be set
// with ID: generated IDs depend on creation order, which a script run may
// not repeat.
func (c *core) name() string {
	if !c.named {
		panic(fmt.Sprintf("ui: a placeholder, patch or Call refers to %s, which needs an ID; set one with .ID(...)", c.id))
	}
	return c.id
}

var refs struct {
	sync.Mutex
	cores []*core
}

// placeholder is a stand-in for "{value:<id>.<path>}" until the component's
// ID is final. Private use characters keep it out of the way of real text.
func (c *core) placeholder(path string) string {
	refs.Lock()
	defer refs.Unlock()
	if c.ref == 0 {
		refs.cores = append(refs.cores, c)
		c.ref = len(refs.cores)
	}
	return "\uE000" + strconv.Itoa(c.ref) + "\uE001" + path + "\uE002"
}

var placeholders = regexp.MustCompile("\uE000(\\d+)\uE001([^\uE002]*)\uE002")

// resolve replaces placeholders in JSON with Alfred's {value:id.path}.
func resolve(data []byte) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	return placeholders.ReplaceAllFunc(data, func(m []byte) []byte {
		parts := placeholders.FindSubmatch(m)
		n, _ := strconv.Atoi(string(parts[1]))
		refs.Lock()
		c := refs.cores[n-1]
		refs.Unlock()
		return []byte("{value:" + c.name() + "." + string(parts[2]) + "}")
	}), nil
}

func (c *core) set(key string, value any) {
	if c.props == nil {
		c.props = map[string]any{}
	}
	c.props[key] = value
}

func (c *core) constrain(key string, value any) {
	if c.target != nil {
		panic("ui: constraints can't be patched")
	}
	if c.constraints == nil {
		c.constraints = map[string]any{}
	}
	c.constraints[key] = value
}

// component gives every component type the shared properties, returning the
// concrete type so its own methods can follow.
type component[T any] struct {
	core
	self  T
	fresh func() T
}

// initer is a component type that can set up its own component fields.
type initer[T any] interface {
	Component
	init(kind string, self T, fresh func() T)
}

func (c *component[T]) init(kind string, self T, fresh func() T) {
	c.kind, c.self, c.fresh = kind, self, fresh
	c.id = nextID(kind)
}

// newComponent sets up self, a new component of kind. fresh makes an empty
// component of the same type for Patch.
func newComponent[T initer[T]](kind string, self T, fresh func() T) T {
	self.init(kind, self, fresh)
	return self
}

func (c *component[T]) base() *core { return &c.core }

// ID names the component, instead of its generated ID.
func (c *component[T]) ID(id string) T {
	if c.target != nil {
		panic("ui: a patch can't change a component's ID")
	}
	c.id, c.named = id, true
	return c.self
}

// Name is the component's ID.
func (c *component[T]) Name() string {
	if c.target != nil {
		return c.target.id
	}
	return c.id
}

func (c *component[T]) mustExist() {
	if c == nil {
		panic("ui: a placeholder or Call refers to a component that's nil; declare components before the actions that refer to them")
	}
}

// Set sets any property, as Alfred's reference names it.
func (c *component[T]) Set(key string, value any) T {
	c.set(key, value)
	return c.self
}

// Get is a placeholder for one of the component's values when an action
// runs, such as Get("text") or Get("selected.uid").
func (c *component[T]) Get(path string) string {
	c.mustExist()
	if c.target != nil {
		return c.target.placeholder(path)
	}
	return c.core.placeholder(path)
}

// Patch starts a change to this component, made by setting properties on
// the patch. Use patches in Update, in a DataView's Row, and in a Cell's
// Init and Selected styles:
//
//	ui.Update(status.Patch().Text("Saved").Colour(ui.System("systemGreenColor")))
func (c *component[T]) Patch() T {
	if c.target != nil {
		return c.self
	}
	p := c.fresh()
	b := any(p).(Component).base()
	b.kind, b.target = c.kind, &c.core
	return p
}

// Hidden hides the component, though it still takes up space.
func (c *component[T]) Hidden(hidden bool) T { return c.Set("hidden", hidden) }

func (c *component[T]) Enabled(enabled bool) T { return c.Set("enabled", enabled) }

func (c *component[T]) Tooltip(tooltip string) T { return c.Set("tooltip", tooltip) }

// Opacity is between 0 and 1.
func (c *component[T]) Opacity(opacity float64) T { return c.Set("opacity", opacity) }

// Focus focuses this component when the interface opens.
func (c *component[T]) Focus() T { return c.Set("focus", true) }

// Background sets the background colour of components that have one.
func (c *component[T]) Background(colour Colour) T { return c.Set("background", colour) }

// Region places the component in a Border layout.
func (c *component[T]) Region(region Region) T {
	c.constrain("region", string(region))
	return c.self
}

// Position aligns the component within its space, such as TopLeft.
func (c *component[T]) Position(position Position) T {
	c.constrain("position", string(position))
	return c.self
}

// Size overrides the component's size. Zero leaves a dimension automatic.
func (c *component[T]) Size(width, height int) T {
	c.constrain("size", map[string]any{"width": width, "height": height})
	return c.self
}

// Width overrides the component's width, leaving its height automatic
// unless Height sets it.
func (c *component[T]) Width(width int) T { return c.sizeKey("width", width) }

// Height overrides the component's height.
func (c *component[T]) Height(height int) T { return c.sizeKey("height", height) }

func (c *component[T]) sizeKey(key string, value int) T {
	size, _ := c.constraints["size"].(map[string]any)
	if size == nil {
		size = map[string]any{}
	}
	size[key] = value
	c.constrain("size", size)
	return c.self
}

// Square sets the component's width and height to size.
func (c *component[T]) Square(size int) T {
	c.constrain("size", size)
	return c.self
}

// Insets adds space around the component.
func (c *component[T]) Insets(top, right, bottom, left int) T {
	c.constrain("insets", map[string]any{"top": top, "right": right, "bottom": bottom, "left": left})
	return c.self
}

// Grid places the component in a Flex layout.
func (c *component[T]) Grid(col, row, colspan, rowspan int) T {
	c.constrain("grid", map[string]any{"col": col, "row": row, "colspan": colspan, "rowspan": rowspan})
	return c.self
}

// Constrain sets any layout constraint, as Alfred's reference names it.
func (c *component[T]) Constrain(key string, value any) T {
	c.constrain(key, value)
	return c.self
}

// On runs actions when trigger fires. Wrap them in Confirm to ask first.
func (c *component[T]) On(trigger Trigger, actions ...Action) T {
	if c.target != nil {
		panic("ui: events can't be patched")
	}
	e := event{trigger: trigger}
	for _, a := range actions {
		if conf, ok := a.(confirmed); ok {
			e.confirm = conf.confirm
			e.actions = append(e.actions, conf.actions...)
			continue
		}
		e.actions = append(e.actions, a)
	}
	c.events = append(c.events, e)
	return c.self
}

func (c *component[T]) OnChanged(actions ...Action) T  { return c.On(Changed, actions...) }
func (c *component[T]) OnActioned(actions ...Action) T { return c.On(Actioned, actions...) }
func (c *component[T]) OnLaunched(actions ...Action) T { return c.On(Launched, actions...) }

// OnDoubleClicked runs actions on a double click.
func (c *component[T]) OnDoubleClicked(actions ...Action) T { return c.On(DoubleClicked, actions...) }

// Trigger is an event that runs actions.
type Trigger string

const (
	Preload       Trigger = "preload"
	Launched      Trigger = "launched"
	Quit          Trigger = "quit"
	Actioned      Trigger = "actioned"
	Changed       Trigger = "changed"
	TypingStopped Trigger = "typingstopped"
	MouseDown     Trigger = "mousedown"
	MouseUp       Trigger = "mouseup"
	Clicked       Trigger = "clicked"
	DoubleClicked Trigger = "doubleclicked"
)

// Region is a place in a Border layout.
type Region string

const (
	Top    Region = "top"
	Right  Region = "right"
	Bottom Region = "bottom"
	Left   Region = "left"
	Centre Region = "centre"
)

// Position aligns a component within its space.
type Position string

const (
	Fit         Position = "fit"
	Centred     Position = "centre"
	TopEdge     Position = "top"
	RightEdge   Position = "right"
	BottomEdge  Position = "bottom"
	LeftEdge    Position = "left"
	TopLeft     Position = "topleft"
	TopRight    Position = "topright"
	BottomLeft  Position = "bottomleft"
	BottomRight Position = "bottomright"
)

// json is the component as it appears in ui.json.
func (c *core) json() map[string]any {
	m := maps.Clone(c.props)
	if m == nil {
		m = map[string]any{}
	}
	m["type"] = c.kind
	m["id"] = c.id
	if c.constraints != nil {
		m["constraints"] = c.constraints
	}
	if c.events != nil {
		events := make([]any, len(c.events))
		for i, e := range c.events {
			ev := map[string]any{"triggers": string(e.trigger), "action": actionList(e.actions)}
			if e.confirm != nil {
				ev["confirm"] = e.confirm
			}
			events[i] = ev
		}
		m["events"] = events
	}
	if c.shortcuts != nil {
		shortcuts := make([]any, len(c.shortcuts))
		for i, sc := range c.shortcuts {
			shortcuts[i] = map[string]any{"shortcut": sc.key, "action": actionList(sc.actions)}
		}
		m["shortcuts"] = shortcuts
	}
	if c.children != nil {
		children := make([]any, len(c.children))
		for i, child := range c.children {
			children[i] = child.base().json()
		}
		m["components"] = children
	}
	return m
}

// patch is a patch's properties, keyed by property.
func (c *core) patch() map[string]any {
	if c.target == nil {
		panic(fmt.Sprintf("ui: %s is a component, not a patch; use %s.Patch()", c.id, c.id))
	}
	return maps.Clone(c.props)
}

// flat writes a patch as "id.property" keys, as dataview rows and cell
// styles need.
func flat(m map[string]any, patches []Component) {
	for _, p := range patches {
		c := p.base()
		id := c.target.name()
		for key, value := range c.patch() {
			m[id+"."+key] = value
		}
	}
}

// Colour is a colour: a system colour, a hex colour, or one for each of
// light and dark mode.
type Colour struct{ v any }

func (c Colour) MarshalJSON() ([]byte, error) { return json.Marshal(c.v) }

// Hex is a colour such as "#FF8800" or "#FF880080" with alpha.
func Hex(hex string) Colour { return Colour{hex} }

// System is a macOS system colour such as "systemBlueColor" or "labelColor",
// or a constant from the document's Colours.
func System(name string) Colour { return Colour{name} }

// Adaptive is light in light mode and dark in dark mode.
func Adaptive(light, dark Colour) Colour {
	return Colour{map[string]any{"light": light.v, "dark": dark.v}}
}

var (
	// NoColour removes a colour, such as a background.
	NoColour        = Colour{}
	LabelColour     = System("labelColor")
	SecondaryLabel  = System("secondaryLabelColor")
	TertiaryLabel   = System("tertiaryLabelColor")
	SeparatorColour = System("separatorColor")
	// SelectedBackground and SelectedText are the colours of the selected
	// result in the user's Alfred theme.
	SelectedBackground = System("alfredSelectedResultBackgroundColor")
	SelectedText       = System("alfredSelectedResultTextColor")
)

// Font is a text style. Size can be points or a name such as "large".
type Font struct {
	Size       any    `json:"size,omitempty"`
	Weight     string `json:"weight,omitempty"`
	Monospaced bool   `json:"monospaced,omitempty"`
}

// Image is an image file, a file's icon or an SF Symbol. The zero Image
// removes an image.
type Image struct {
	Type string         `json:"type,omitempty"`
	Src  string         `json:"src,omitempty"`
	Size map[string]int `json:"size,omitempty"`
}

func (i Image) MarshalJSON() ([]byte, error) {
	if i.Src == "" {
		return []byte("null"), nil
	}
	type plain Image
	return json.Marshal(plain(i))
}

// NoImage removes an image.
var NoImage = Image{}

// ImageFile is the image at path, relative to the .alfredui package or absolute.
func ImageFile(path string) Image { return Image{Type: "file", Src: path} }

// FileIcon is the Finder icon of the file at path.
func FileIcon(path string) Image { return Image{Type: "fileicon", Src: path} }

// Symbol is an SF Symbol, such as "star.fill".
func Symbol(name string) Image { return Image{Type: "symbol", Src: name} }

// Sized sets the image's preferred size. Zero derives a dimension from the other.
func (i Image) Sized(width, height int) Image {
	i.Size = map[string]int{"width": width, "height": height}
	return i
}
