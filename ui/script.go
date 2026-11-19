package ui

import (
	"fmt"
	"reflect"
	"strconv"
)

// runner is a script, as a document collects them.
type runner interface {
	scriptName() string
	call(args []string) (any, error)
}

// Script is a Go function that the interface runs, passing values from the
// interface as the fields of A, a struct of string, bool and int fields, in
// order. It returns the Changes to make. Run the script from an event with Run:
//
//	type searchArgs struct{ Query, Kind string }
//	search := ui.NewScript("search", func(a searchArgs) (ui.Changes, error) { ... })
//	query.OnChanged(search.Run(searchArgs{Query: query.TextValue(), Kind: kinds.SelectedValue()}))
//
// alfred.UI packages each script the document uses as scripts/<name>.
type Script[A any] struct {
	name   string
	fn     func(A) (Changes, error)
	async  bool
	fields []int
}

// NewScript makes fn a script called name.
func NewScript[A any](name string, fn func(A) (Changes, error)) *Script[A] {
	t := reflect.TypeFor[A]()
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("ui: script %s takes %s; it must take a struct", name, t))
	}
	s := &Script[A]{name: name, fn: fn}
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		switch f.Type.Kind() {
		case reflect.String, reflect.Bool, reflect.Int, reflect.Int64:
		default:
			panic(fmt.Sprintf("ui: script %s field %s is %s; use string, bool or int", name, f.Name, f.Type))
		}
		s.fields = append(s.fields, i)
	}
	return s
}

// Async runs the script in the background, so the interface stays
// responsive. Its actions apply when it finishes, when the interface may
// have changed.
func (s *Script[A]) Async() *Script[A] {
	s.async = true
	return s
}

func (s *Script[A]) scriptName() string { return s.name }

// Run is an action that runs the script with args, whose string fields can
// hold placeholders such as TextField.TextValue.
func (s *Script[A]) Run(args A) Action {
	v := reflect.ValueOf(args)
	values := make([]string, len(s.fields))
	for i, f := range s.fields {
		values[i] = fmt.Sprint(v.Field(f).Interface())
	}
	return scriptRun{script: s, args: values}
}

// Call runs the script as Alfred does, with the args a Run action passes.
// Alfred passes "<null>" for unset values, such as a list with nothing
// selected; they arrive as zero values.
func (s *Script[A]) Call(args []string) (Changes, error) {
	var a A
	v := reflect.ValueOf(&a).Elem()
	for i, f := range s.fields {
		if i >= len(args) || args[i] == "<null>" || args[i] == "" {
			continue
		}
		field := v.Field(f)
		switch field.Kind() {
		case reflect.String:
			field.SetString(args[i])
		case reflect.Bool:
			b, err := strconv.ParseBool(args[i])
			if err != nil {
				return Changes{}, fmt.Errorf("script %s: %s: %w", s.name, v.Type().Field(f).Name, err)
			}
			field.SetBool(b)
		default:
			n, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				return Changes{}, fmt.Errorf("script %s: %s: %w", s.name, v.Type().Field(f).Name, err)
			}
			field.SetInt(n)
		}
	}
	return s.fn(a)
}

func (s *Script[A]) call(args []string) (any, error) {
	actions, err := s.Call(args)
	if err != nil || len(actions.commands) == 0 {
		return nil, err
	}
	return actions, nil
}

type scriptRun struct {
	script interface {
		runner
		isAsync() bool
	}
	args []string
}

func (s *Script[A]) isAsync() bool { return s.async }

func (r scriptRun) action() map[string]any {
	m := map[string]any{"type": "script", "src": "scripts/" + r.script.scriptName(), "args": r.args}
	if r.script.isAsync() {
		m["async"] = true
	}
	return m
}

func (r scriptRun) uses() []runner { return []runner{r.script} }
