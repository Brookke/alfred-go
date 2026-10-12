# alfred-go

Declare [Alfred](https://www.alfredapp.com) workflows in Go, in the style of SwiftUI or React.

```go
func main() {
	alfred.Main(alfred.Workflow("Search",
		alfred.ScriptFilter("s", search).
			Title("Search the web").
			Then(alfred.OpenURL()).
			On(alfred.Cmd, "Copy URL", alfred.Copy()),
	).BundleID("com.example.search"))
}

func search(query string) alfred.Output {
	return alfred.Items{
		alfred.Item("Search Google for " + query).
			Subtitle("Google").
			Arg("https://www.google.com/search?q=" + url.QueryEscape(query)),
	}
}
```

`go run . install` builds the workflow and opens it in Alfred to import it.

## How it works

The program is both the build tool and the workflow. `build` writes an `.alfredworkflow` containing the generated `info.plist` and a copy of the program. Each script filter or `Run` action in the plist is one line, `exec ./workflow run <uid> "$1"`, so Alfred calls back into your Go function. There are no shell scripts and no runtime dependencies. The build is for your Mac's architecture.

- **Results are values.** `Item(...)` modifiers such as `.Subtitle`, `.Arg` and `.Mod` return changed copies, so one result can be shared and varied safely, as with SwiftUI views.
- **The graph is composition.** `Then` connects nodes, and `On(modifier, subtitle, ...)` connects them for a held key. Passing the same node to several inputs gives one shared object in Alfred.
- **Handlers are plain functions.** A `Filter` is `func(query string) alfred.Output` and returns `Items`, or a `Response` for `Rerun` and variables. Test them like any other function; see `examples/search`. A panicking filter shows an error result instead of nothing.
- **Layout is automatic.** Nodes are placed in columns by their distance from an input. UIDs come from the graph's shape (`s`, `s.openurl`); set `.ID(...)` to pin one.

## Nodes

| Function | Alfred object |
| --- | --- |
| `ScriptFilter(keyword, filter)` | Script Filter backed by a Go function |
| `Keyword(keyword)` | Keyword input |
| `OpenURL()` | Open URL, using its input |
| `Copy()`, `Paste()` | Copy to Clipboard, optionally pasting |
| `Notify(title)` | Post Notification |
| `Run(handler)` | Run Script backed by a Go function; its output passes on |
| `Raw(kind, version, config)` | Any other object, as in an exported `info.plist` |

Inputs take `.Title`, `.Subtitle`, `.NoSpace`, `.ArgumentRequired` and `.NoArgument`. Actions take `.Set(key, value)` for config this package doesn't model.

## Commands

`alfred.Main` gives your program these subcommands. Run them from your workflow's directory:

```
go run . build [-o dir]     write the .alfredworkflow to dir (default dist)
go run . install [-o dir]   build, then open the workflow in Alfred to import it
go run . plist              print info.plist
go run . run <uid> [query]  call a handler, as Alfred does
```

Alfred uses `run` itself. Calling it directly is a quick way to check a handler's output.

`go test ./...` checks the generated plist with `plutil` and builds a real package.
