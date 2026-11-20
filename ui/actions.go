package ui

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
)

// Action is what an event or shortcut runs: Changes made with Apply, a
// script's Run, or a component function with Call.
type Action interface {
	action() map[string]any
	// uses lists the scripts the action runs.
	uses() []runner
}

func actionList(actions []Action) any {
	list := make([]any, len(actions))
	for i, a := range actions {
		list[i] = a.action()
	}
	if len(list) == 1 {
		return list[0]
	}
	return list
}

// Changes are what scripts return and events can run without a script:
// updates to the interface, and other commands such as Finish. They are
// Alfred's JSON actions.
type Changes struct {
	commands []Command
}

// Command is one change, such as Update or Finish.
type Command interface {
	apply(commands map[string]any)
}

// Apply combines commands into Changes. Alfred applies them in its own
// order: remove, insert, update, replace, then the rest, with finish last.
// Commands resolve the components they refer to when they're written, so
// those components can be named afterwards.
func Apply(commands ...Command) Changes {
	if commands == nil {
		commands = []Command{}
	}
	return Changes{commands: commands}
}

// None is no changes, for a script with nothing to change.
var None = Changes{}

func (a Changes) build() map[string]any {
	m := map[string]any{}
	for _, c := range a.commands {
		c.apply(m)
	}
	return m
}

func (a Changes) action() map[string]any {
	return map[string]any{"type": "json", "src": a.build()}
}

func (a Changes) uses() []runner { return nil }

func (a Changes) MarshalJSON() (data []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	if data, err = jsonMarshal(a.build()); err != nil {
		return nil, err
	}
	return resolve(data)
}

type commandFunc func(map[string]any)

func (f commandFunc) apply(m map[string]any) { f(m) }

func sub(m map[string]any, key string) map[string]any {
	if s, ok := m[key].(map[string]any); ok {
		return s
	}
	s := map[string]any{}
	m[key] = s
	return s
}

// Update applies patches to their components.
func Update(patches ...Component) Command {
	return commandFunc(func(m map[string]any) {
		ids := sub(sub(m, "update"), "ids")
		for _, p := range patches {
			c := p.base()
			props := c.patch()
			id := c.target.name()
			if existing, ok := ids[id].(map[string]any); ok {
				maps.Copy(existing, props)
			} else {
				ids[id] = props
			}
		}
	})
}

// Relayout recalculates the layout after an Update that changes sizes.
func Relayout() Command {
	return commandFunc(func(m map[string]any) { sub(m, "update")["relayout"] = true })
}

// SetVar sets an interface variable, read with Var.
func SetVar(name, value string) Command {
	return commandFunc(func(m map[string]any) { sub(m, "variables")[name] = value })
}

// Var is a placeholder for an interface variable when an action runs.
func Var(name string) string { return "{var:" + name + "}" }

// CopyText copies text to the clipboard.
func CopyText(text string) Command {
	return commandFunc(func(m map[string]any) { m["copy"] = map[string]any{"type": "text", "src": text} })
}

// Open opens files or URLs.
func Open(paths ...string) Command {
	return commandFunc(func(m map[string]any) { m["open"] = map[string]any{"src": paths} })
}

// Sound plays a system sound, such as "Funk".
func Sound(name string) Command {
	return commandFunc(func(m map[string]any) { m["sound"] = map[string]any{"src": name} })
}

// RawCommand is any other JSON action, as Alfred's reference names it.
func RawCommand(name string, value any) Command {
	return commandFunc(func(m map[string]any) { m[name] = value })
}

// FinishCommand closes the interface, passing args and variables on to the
// workflow objects connected to it.
type FinishCommand struct {
	args []string
	vars map[string]string
}

// Finish closes the interface and passes args to the next workflow objects.
func Finish(args ...string) *FinishCommand { return &FinishCommand{args: args} }

// Var sets a workflow variable for the next objects.
func (f *FinishCommand) Var(name, value string) *FinishCommand {
	if f.vars == nil {
		f.vars = map[string]string{}
	}
	f.vars[name] = value
	return f
}

func (f *FinishCommand) apply(m map[string]any) {
	finish := map[string]any{}
	switch len(f.args) {
	case 0:
	case 1:
		finish["args"] = f.args[0]
	default:
		finish["args"] = f.args
	}
	if f.vars != nil {
		finish["variables"] = f.vars
	}
	m["finish"] = finish
}

func (f *FinishCommand) action() map[string]any { return Apply(f).action() }
func (f *FinishCommand) uses() []runner         { return nil }

type call struct {
	target    *core
	functions []string
}

// Call runs functions on a component, such as Call(list, ui.SelectDown).
func Call(c Component, functions ...string) Action {
	if c == nil || reflect.ValueOf(c).IsNil() {
		panic("ui: Call refers to a component that's nil; declare components before the actions that refer to them")
	}
	return call{target: c.base(), functions: functions}
}

func (c call) action() map[string]any {
	var args any = c.functions
	if len(c.functions) == 1 {
		args = c.functions[0]
	}
	return map[string]any{"type": "function", "src": c.target.name(), "args": args}
}

func (c call) uses() []runner { return nil }

type logAction string

// Log writes message to the interface's log.
func Log(message string) Action { return logAction(message) }

func (l logAction) action() map[string]any {
	return map[string]any{"type": "log", "src": "", "args": string(l)}
}
func (l logAction) uses() []runner { return nil }

type confirmed struct {
	confirm map[string]any
	actions []Action
}

// Confirm asks before running actions, with a dialog showing message and a
// confirm button labelled button.
func Confirm(message, button string, actions ...Action) Action {
	return confirmed{
		confirm: map[string]any{"message": message, "confirmbuttontext": button},
		actions: actions,
	}
}

func (c confirmed) action() map[string]any { panic("ui: Confirm can only be used directly in On") }

func (c confirmed) uses() []runner {
	var r []runner
	for _, a := range c.actions {
		r = append(r, a.uses()...)
	}
	return r
}

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }
