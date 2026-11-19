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
| `UI(name, doc)` | Alfred UI declared in Go with package `ui` |
| `AlfredUI(name, files)` | Alfred UI from a `.alfredui` folder, with `.Script(name, handler)` for Go-backed script actions |
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

[Alfred UI](https://www.alfredapp.com/help/alfredui/) (Alfred 6) builds native interfaces. Package `ui` declares them in Go, and `alfred.UI` adds one to a workflow:

```go
type searchArgs struct{ Query string }

func main() {
	query := ui.TextField().ID("query").Placeholder("Search notes").Focus()
	title := ui.Label().ID("title")
	results := ui.DataView().Cell(ui.Cell(ui.HStack(title).Padding(8)).Height(36).
		Selected(title.Patch().Colour(ui.SelectedText))).ID("results")

	search := ui.NewScript("search", func(a searchArgs) (ui.Changes, error) {
		var rows []ui.RowData
		for _, n := range find(a.Query) {
			rows = append(rows, ui.Row(n.ID, title.Patch().Text(n.Title)).With("path", n.Path))
		}
		return ui.Apply(ui.Update(results.Patch().Rows(rows...))), nil
	})

	doc := ui.Window(ui.Border(
		query.Region(ui.Top).OnChanged(search.Run(searchArgs{Query: query.TextValue()})),
		results.Region(ui.Centre).OnActioned(ui.Finish(results.Selected("path"))),
	)).Size(600, 400)

	alfred.Main(alfred.Workflow("Notes",
		alfred.Keyword("notes").NoArgument().Then(alfred.UI("notes.alfredui", doc).Then(alfred.OpenURL())),
	).BundleID("com.example.notes"))
}
```

- **Components are values.** Each has methods for its properties, and `.Set(key, value)` covers the rest of [Alfred's reference](https://www.alfredapp.com/help/alfredui/components/). Lay them out with `Border` and `.Region`, or with `HStack`, `VStack`, `Stack`, `Grid`, `Flex` and `Form`.
- **Patches change them.** `title.Patch().Text("…")` is a change to `title`, made with the same methods. Use patches in `ui.Update`, in a `DataView`'s `ui.Row`, and in a `Cell`'s `Init` and `Selected` styles. Components and data share one set of names, so a typo is a compile error rather than a silently ignored key.
- **Scripts are typed Go functions.** A `ui.NewScript` takes a struct, and `Run` fills its fields with placeholders such as `query.TextValue()`. Alfred calls the script back with those values, and `"<null>"` arrives as a zero value. The script returns `ui.Changes`, made with `ui.Apply(ui.Update(...), ...)`. Each script the document uses is packaged as `scripts/<name>`, a launcher that calls back into your program. With `Window(...).ErrorLabel(label)`, an error a script returns appears in that label in red.
- **Finish hands off to the workflow.** The args of `ui.Finish` pass to the nodes connected with `Then`, and `.Var` sets workflow variables.

Give an ID with `.ID("results")` to every component that a placeholder, patch or `ui.Call` refers to. Building the document reports any it's missing. The others get IDs from their type and creation order, such as `label2`. References resolve when the document is written, so a component can be named and configured after actions refer to it; it only has to exist. A larger UI can declare its components first and then lay them out and wire them in any order. `.External()` opens the UI in its own window, and `.Stack()` makes ⎋ go back instead of closing. `.Files(fsys)` adds images and other files to the package.

For a UI written as JSON, `alfred.AlfredUI(name, files)` packages a `.alfredui` folder instead, and `.Script(name, handler)` gives its script actions Go handlers.

`alfred.DataDir()` and `alfred.CacheDir()` are the workflow's folders for data it keeps and data it can rebuild. Outside Alfred, they're where Alfred would put them.

Keyword inputs take `.Title`, `.Subtitle`, `.NoSpace`, `.ArgumentRequired` and `.NoArgument`. Actions take `.Set(key, value)` for config this package doesn't model.

## Commands

`alfred.Main` gives your program these subcommands. Run them from your workflow's directory:

```
go run . build [-o dir]     write the .alfredworkflow to dir (default dist)
go run . install [-o dir]   build, then open the workflow in Alfred to import it
go run . dev [-log]         build, install in place over the last copy, and reload it
go run . plist              print info.plist
go run . run <uid> [query]  call a handler, as Alfred does
go run . ui <uid> <script> [args...]
                            call an Alfred UI script, as Alfred does
```

`dev` is for working on a workflow: it replaces the installed copy with the same bundle ID instead of importing another, and has Alfred reload it. With `-log`, Alfred UIs log everything to a file, and `dev` prints its path.

Alfred uses `run` itself. Calling it directly is a quick way to check a handler's output.

`go test ./...` checks the generated plist with `plutil` and builds a real package.
