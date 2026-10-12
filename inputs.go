package alfred

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
)

const (
	argumentRequired = 0
	argumentOptional = 1
	argumentNone     = 2
)

// input holds the settings shared by inputs triggered with a keyword.
type input[T any] struct {
	node[T]
	keyword, title, subtitle string
	noSpace                  bool
	argument                 int
}

// Title is shown for the keyword before a query is typed.
func (i *input[T]) Title(title string) T {
	i.title = title
	return i.self
}

// Subtitle is shown beneath Title.
func (i *input[T]) Subtitle(subtitle string) T {
	i.subtitle = subtitle
	return i.self
}

// NoSpace lets the query follow the keyword directly, so "pr123" works as
// well as "pr 123".
func (i *input[T]) NoSpace() T {
	i.noSpace = true
	return i.self
}

// ArgumentRequired waits for a query before running. By default the query is
// optional.
func (i *input[T]) ArgumentRequired() T {
	i.argument = argumentRequired
	return i.self
}

// NoArgument triggers on the bare keyword, without a query.
func (i *input[T]) NoArgument() T {
	i.argument = argumentNone
	return i.self
}

func (i *input[T]) config() map[string]any {
	return map[string]any{
		"keyword": i.keyword, "subtext": i.subtitle,
		"withspace": !i.noSpace, "argumenttype": i.argument,
	}
}

// Filter renders results for the query typed after a script filter's keyword.
type Filter func(query string) Output

// ScriptFilterNode is a keyword whose results are rendered by a Go function.
type ScriptFilterNode struct {
	input[*ScriptFilterNode]
	running       string
	alfredFilters bool
	filter        Filter
}

// ScriptFilter shows the results filter renders for whatever follows keyword.
// The actioned result's Arg is passed to the nodes connected with Then and On.
func ScriptFilter(keyword string, filter Filter) *ScriptFilterNode {
	s := &ScriptFilterNode{filter: filter}
	s.self, s.label, s.keyword, s.argument = s, keyword, keyword, argumentOptional
	return s
}

// Running is the subtitle shown while the filter runs.
func (s *ScriptFilterNode) Running(subtitle string) *ScriptFilterNode {
	s.running = subtitle
	return s
}

// AlfredFilters lets Alfred filter the results as the user types, instead of
// the filter receiving the query. See ItemView.Match.
func (s *ScriptFilterNode) AlfredFilters() *ScriptFilterNode {
	s.alfredFilters = true
	return s
}

func (s *ScriptFilterNode) object(uid string) (string, int, map[string]any) {
	config := s.config()
	maps.Copy(config, map[string]any{
		"title": s.title, "runningsubtext": s.running,
		"alfredfiltersresults": s.alfredFilters, "alfredfiltersresultsmatchmode": 0,
		"script": dispatch(uid), "scriptargtype": 1, "scriptfile": "", "type": 0, "escaping": 102,
		"argumenttrimmode": 0, "queuemode": 1, "queuedelaymode": 0, "queuedelaycustom": 3,
		"queuedelayimmediatelyalinitially": true,
	})
	return "alfred.workflow.input.scriptfilter", 3, config
}

func (s *ScriptFilterNode) run(query string, out io.Writer) error {
	return json.NewEncoder(out).Encode(render(s.filter, query))
}

// render calls filter. A panic becomes an error result.
func render(filter Filter, query string) (response Response) {
	defer func() {
		if r := recover(); r != nil {
			response = Error(fmt.Errorf("%v", r)).Response()
		}
	}()
	if filter == nil {
		return Response{}
	}
	if out := filter(query); out != nil {
		return out.Response()
	}
	return Response{}
}

// KeywordNode is a keyword that passes its query straight to its connections.
type KeywordNode struct {
	input[*KeywordNode]
}

// Keyword passes whatever follows keyword to the nodes connected with Then.
func Keyword(keyword string) *KeywordNode {
	k := &KeywordNode{}
	k.self, k.label, k.keyword, k.argument = k, keyword, keyword, argumentOptional
	return k
}

func (k *KeywordNode) object(string) (string, int, map[string]any) {
	config := k.config()
	config["text"] = k.title
	return "alfred.workflow.input.keyword", 1, config
}
