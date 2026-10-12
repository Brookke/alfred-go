// Package alfred declares Alfred workflows in Go, in the style of SwiftUI or React.
//
// A workflow is a graph of nodes. Inputs such as ScriptFilter lead, through
// Then and On, to actions such as OpenURL. Script filters and Run actions are
// ordinary Go functions: the compiled program both builds the workflow and is
// the code Alfred runs, so there are no shell scripts to maintain.
//
//	func main() {
//		alfred.Main(alfred.Workflow("Hello",
//			alfred.ScriptFilter("hi", greet).Then(alfred.OpenURL()),
//		).BundleID("com.example.hello"))
//	}
//
//	func greet(query string) alfred.Output {
//		return alfred.Items{
//			alfred.Item("Hello " + query).Arg("https://example.com"),
//		}
//	}
package alfred

import (
	"io"
	"strings"
)

// Modifier is a key held while actioning a result. Combine them with |.
type Modifier int

const (
	None  Modifier = 0
	Shift Modifier = 1 << 17
	Ctrl  Modifier = 1 << 18
	Alt   Modifier = 1 << 19
	Cmd   Modifier = 1 << 20
	Fn    Modifier = 1 << 23
)

// key is the modifier's name in script filter results, such as "cmd+shift".
func (m Modifier) key() string {
	var names []string
	for _, k := range []struct {
		mod  Modifier
		name string
	}{{Cmd, "cmd"}, {Alt, "alt"}, {Ctrl, "ctrl"}, {Shift, "shift"}, {Fn, "fn"}} {
		if m&k.mod != 0 {
			names = append(names, k.name)
		}
	}
	return strings.Join(names, "+")
}

// Node is an object in the workflow graph, created by functions such as
// ScriptFilter and OpenURL.
type Node interface {
	base() *core
	object(uid string) (kind string, version int, config map[string]any)
}

// runner is a node backed by a Go function that Alfred calls at runtime.
type runner interface {
	run(arg string, out io.Writer) error
}

type core struct {
	id, label string
	links     []link
}

type link struct {
	mod      Modifier
	subtitle string
	to       Node
}

// node gives every node type the same chainable connection methods, returning
// the concrete type so its own modifiers can follow.
type node[T any] struct {
	core
	self T
}

func (n *node[T]) base() *core { return &n.core }

// Then connects this node to targets, which receive its output.
func (n *node[T]) Then(targets ...Node) T { return n.On(None, "", targets...) }

// On connects targets that run when the result is actioned with mod held.
// subtitle replaces the result's subtitle while mod is held.
func (n *node[T]) On(mod Modifier, subtitle string, targets ...Node) T {
	for _, target := range targets {
		n.links = append(n.links, link{mod, subtitle, target})
	}
	return n.self
}

// ID fixes the node's object UID. By default UIDs derive from the node's
// position in the graph, which is stable while the graph's shape is.
func (n *node[T]) ID(id string) T {
	n.id = id
	return n.self
}
