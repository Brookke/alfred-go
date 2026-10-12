package alfred

import (
	"encoding/json"
	"maps"
	"time"
)

// Output is what a Filter renders.
type Output interface {
	Response() Response
}

// Response is a complete script filter result.
type Response struct {
	Items Items
	// Rerun asks Alfred to run the filter again after this long, between 0.1
	// and 5 seconds, for results that are still loading.
	Rerun time.Duration
	// Vars are passed to the next nodes as workflow variables.
	Vars map[string]string
}

func (r Response) Response() Response { return r }

func (r Response) MarshalJSON() ([]byte, error) {
	items := r.Items
	if items == nil {
		items = Items{}
	}
	return json.Marshal(struct {
		Items Items             `json:"items"`
		Rerun float64           `json:"rerun,omitempty"`
		Vars  map[string]string `json:"variables,omitempty"`
	}{items, r.Rerun.Seconds(), r.Vars})
}

// Items is a list of results, shown in order.
type Items []ItemView

func (i Items) Response() Response { return Response{Items: i} }

// Error renders err as a single result that can't be actioned.
func Error(err error) Items {
	return Items{Item("Something went wrong").Subtitle(err.Error()).Invalid()}
}

// ItemView is one result. Its modifiers return changed copies, so a view can
// be shared and varied freely.
type ItemView struct{ data item }

type item struct {
	UID          string            `json:"uid,omitempty"`
	Title        string            `json:"title"`
	Subtitle     string            `json:"subtitle,omitempty"`
	Arg          string            `json:"arg,omitempty"`
	Valid        *bool             `json:"valid,omitempty"`
	Autocomplete string            `json:"autocomplete,omitempty"`
	Match        string            `json:"match,omitempty"`
	QuickLook    string            `json:"quicklookurl,omitempty"`
	Icon         *icon             `json:"icon,omitempty"`
	Text         *text             `json:"text,omitempty"`
	Mods         map[string]mod    `json:"mods,omitempty"`
	Vars         map[string]string `json:"variables,omitempty"`
}

type icon struct {
	Type string `json:"type,omitempty"`
	Path string `json:"path"`
}

type text struct {
	Copy      string `json:"copy,omitempty"`
	LargeType string `json:"largetype,omitempty"`
}

type mod struct {
	Valid    bool   `json:"valid"`
	Arg      string `json:"arg,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
}

func Item(title string) ItemView { return ItemView{item{Title: title}} }

func (i ItemView) MarshalJSON() ([]byte, error) { return json.Marshal(i.data) }

func (i ItemView) Subtitle(subtitle string) ItemView {
	i.data.Subtitle = subtitle
	return i
}

// Arg is passed to the connected nodes when the result is actioned.
func (i ItemView) Arg(arg string) ItemView {
	i.data.Arg = arg
	return i
}

// UID lets Alfred learn which results are chosen most and rank them first.
func (i ItemView) UID(uid string) ItemView {
	i.data.UID = uid
	return i
}

// Autocomplete replaces the query when the result is tab-completed.
func (i ItemView) Autocomplete(query string) ItemView {
	i.data.Autocomplete = query
	return i
}

// Match is the text Alfred filters on with AlfredFilters, instead of the title.
func (i ItemView) Match(match string) ItemView {
	i.data.Match = match
	return i
}

func (i ItemView) QuickLook(url string) ItemView {
	i.data.QuickLook = url
	return i
}

// Icon's path is relative to the workflow.
func (i ItemView) Icon(path string) ItemView {
	i.data.Icon = &icon{Path: path}
	return i
}

// FileIcon shows the Finder icon of the file at path.
func (i ItemView) FileIcon(path string) ItemView {
	i.data.Icon = &icon{Type: "fileicon", Path: path}
	return i
}

// Copy is the text copied with ⌘C, instead of Arg.
func (i ItemView) Copy(copy string) ItemView {
	t := i.text()
	t.Copy = copy
	i.data.Text = &t
	return i
}

// LargeType is the text shown with ⌘L, instead of the title.
func (i ItemView) LargeType(largeType string) ItemView {
	t := i.text()
	t.LargeType = largeType
	i.data.Text = &t
	return i
}

func (i ItemView) text() text {
	if i.data.Text == nil {
		return text{}
	}
	return *i.data.Text
}

// Invalid stops the result being actioned, for hints and errors.
func (i ItemView) Invalid() ItemView {
	valid := false
	i.data.Valid = &valid
	return i
}

// Mod replaces Arg and the subtitle while m is held.
func (i ItemView) Mod(m Modifier, subtitle, arg string) ItemView {
	mods := maps.Clone(i.data.Mods)
	if mods == nil {
		mods = map[string]mod{}
	}
	mods[m.key()] = mod{Valid: true, Arg: arg, Subtitle: subtitle}
	i.data.Mods = mods
	return i
}

// Var sets a workflow variable for the next nodes when the result is actioned.
func (i ItemView) Var(name, value string) ItemView {
	vars := maps.Clone(i.data.Vars)
	if vars == nil {
		vars = map[string]string{}
	}
	vars[name] = value
	i.data.Vars = vars
	return i
}
