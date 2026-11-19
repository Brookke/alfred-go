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
| `External(id)` | External Trigger, called with AppleScript; `.URLHandler()` also allows `alfred://runtrigger/` URLs |
| `Hotkey(mods, key)` | Hotkey trigger, such as `Hotkey(alfred.Cmd\|alfred.Alt, "S")`; `""` leaves the key for you to record in Alfred; `.OnlyIn(bundleIDs...)` limits it to those apps |
| `OpenURL()` | Open URL, using its input |
| `Copy()`, `Paste()` | Copy to Clipboard, optionally pasting |
| `LaunchApps(paths...)` | Launch Apps / Files; `.Set("toggle", true)` hides an app that's already frontmost |
| `Notify(title)` | Post Notification |
| `Run(handler)` | Run Script backed by a Go function; its output passes on |
| `AlfredUI(name, files)` | Alfred UI from a `.alfredui` package, with `.Script(name, handler)` for Go-backed script actions |
| `Raw(kind, version, config)` | Any other object, as in an exported `info.plist` |

A hotkey that opens an app:

```go
alfred.Hotkey(alfred.Cmd|alfred.Alt, "S").Then(alfred.LaunchApps("/Applications/Safari.app"))
```

Alfred removes a hotkey's key and modifiers when it imports a workflow, so after `install` record the shortcut again on the hotkey in Alfred. `.OnlyIn` and other settings are kept.

`.OnlyIn(bundleIDs...)` limits a hotkey to when one of those apps is frontmost. A Chrome-only shortcut that copies the current tab's URL, like Arc's:

```go
alfred.Hotkey(alfred.Cmd|alfred.Shift, "C").OnlyIn("com.google.Chrome").
	Then(alfred.Run(chromeURL).Then(alfred.Copy(), alfred.Notify("URL copied")))

func chromeURL(string) (string, error) {
	out, err := exec.Command("osascript", "-e",
		`tell application "Google Chrome" to get URL of active tab of front window`).Output()
	return strings.TrimSpace(string(out)), err
}
```

## Alfred UI

[Alfred UI](https://www.alfredapp.com/help/alfredui/) (Alfred 6) builds native interfaces from a `ui.json`. Embed the package and give its script actions Go handlers:

```go
//go:embed all:notes.alfredui
var notesUI embed.FS

func main() {
	files, _ := fs.Sub(notesUI, "notes.alfredui")
	ui := alfred.AlfredUI("notes.alfredui", files).
		Script("search", search).
		Then(alfred.Paste())
	alfred.Main(alfred.Workflow("Notes", alfred.Keyword("notes").NoArgument().Then(ui)).
		BundleID("com.example.notes"))
}

// search runs for {"type": "script", "src": "scripts/search", "args": ["{value:query.text}"]}.
func search(args []string) (any, error) {
	return alfred.UIActions{"update": map[string]any{"ids": map[string]any{
		"results": map[string]any{"data": map[string]any{"src": find(args[0])}},
	}}}, nil
}
```

Each `Script` is packaged as `scripts/<name>` in the package, a launcher that calls back into your program. Whatever the handler returns is encoded as [JSON actions](https://www.alfredapp.com/help/alfredui/actions/json/). The args of the UI's `finish` action pass to the nodes connected with `Then`. `.External()` opens the UI in its own window, and `.Stack()` makes ⎋ go back instead of closing. Check a script with `go run . ui <uid> <script> [args...]`.

Keyword inputs take `.Title`, `.Subtitle`, `.NoSpace`, `.ArgumentRequired` and `.NoArgument`. Actions take `.Set(key, value)` for config this package doesn't model.

## Commands

`alfred.Main` gives your program these subcommands. Run them from your workflow's directory:

```
go run . build [-o dir]     write the .alfredworkflow to dir (default dist)
go run . install [-o dir]   build, then open the workflow in Alfred to import it
go run . plist              print info.plist
go run . run <uid> [query]  call a handler, as Alfred does
go run . ui <uid> <script> [args...]
                            call an Alfred UI script, as Alfred does
```

Alfred uses `run` itself. Calling it directly is a quick way to check a handler's output.

`go test ./...` checks the generated plist with `plutil` and builds a real package.
