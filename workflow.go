package alfred

import (
	"archive/zip"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// binaryName is the program's file name inside the built workflow.
const binaryName = "workflow"

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// WorkflowView is a whole workflow.
type WorkflowView struct {
	name, bundleID, version, author, description, readme, category, website, icon string

	vars   map[string]string
	inputs []Node
}

// Workflow declares a workflow whose graph starts at inputs. Nodes connected
// to the inputs are included automatically.
func Workflow(name string, inputs ...Node) *WorkflowView {
	return &WorkflowView{name: name, inputs: inputs}
}

// BundleID identifies the workflow, such as "com.example.hello". Required.
func (w *WorkflowView) BundleID(id string) *WorkflowView { w.bundleID = id; return w }

func (w *WorkflowView) Version(version string) *WorkflowView { w.version = version; return w }

func (w *WorkflowView) Author(author string) *WorkflowView { w.author = author; return w }

func (w *WorkflowView) Description(description string) *WorkflowView {
	w.description = description
	return w
}

// Readme is shown in the workflow's About panel.
func (w *WorkflowView) Readme(readme string) *WorkflowView { w.readme = readme; return w }

// Category groups the workflow in Alfred, such as "Tools" or "Productivity".
func (w *WorkflowView) Category(category string) *WorkflowView { w.category = category; return w }

func (w *WorkflowView) Website(url string) *WorkflowView { w.website = url; return w }

// Icon is a PNG file packaged as the workflow's icon.
func (w *WorkflowView) Icon(path string) *WorkflowView { w.icon = path; return w }

// Var sets a workflow environment variable, which handlers read with os.Getenv.
func (w *WorkflowView) Var(name, value string) *WorkflowView {
	if w.vars == nil {
		w.vars = make(map[string]string)
	}
	w.vars[name] = value
	return w
}

// Main runs the workflow program's command line (see usage) and exits.
func Main(w *WorkflowView) {
	os.Exit(w.command(os.Args[1:], os.Stdout, os.Stderr))
}

const usage = `usage:
  %[1]s build [-o dir]     write the .alfredworkflow to dir (default dist)
  %[1]s install [-o dir]   build, then open the workflow in Alfred to import it
  %[1]s plist              print info.plist
  %[1]s run <uid> [query]  call a handler, as Alfred does
`

func (w *WorkflowView) command(args []string, stdout, stderr io.Writer) int {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}
	var err error
	switch name {
	case "run":
		if len(args) < 2 {
			err = errors.New("usage: run <uid> [query]")
			break
		}
		query := ""
		if len(args) > 2 {
			query = args[2]
		}
		err = w.run(args[1], query, stdout)
	case "plist":
		var p map[string]any
		if p, err = w.plist(); err == nil {
			err = encodePlist(stdout, p)
		}
	case "build", "install":
		flags := flag.NewFlagSet(name, flag.ContinueOnError)
		flags.SetOutput(stderr)
		dir := flags.String("o", "dist", "output directory")
		if err := flags.Parse(args[1:]); errors.Is(err, flag.ErrHelp) {
			return 0
		} else if err != nil {
			return 2
		}
		var path string
		if path, err = w.build(*dir); err == nil {
			fmt.Fprintln(stdout, "Built", path)
			if name == "install" {
				err = exec.Command("open", path).Run()
			}
		}
	default:
		fmt.Fprintf(stderr, usage, filepath.Base(os.Args[0]))
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

// dispatch is the script Alfred runs to call back into this program.
func dispatch(uid string) string {
	return fmt.Sprintf(`exec ./%s run %s "$1"`, binaryName, uid)
}

func (w *WorkflowView) run(uid, arg string, out io.Writer) error {
	g, err := w.graph()
	if err != nil {
		return err
	}
	for _, n := range g.nodes {
		if g.uids[n] != uid {
			continue
		}
		if r, ok := n.(runner); ok {
			return r.run(arg, out)
		}
		return fmt.Errorf("%q has no Go handler", uid)
	}
	return fmt.Errorf("no node has UID %q", uid)
}

type graph struct {
	nodes []Node
	uids  map[Node]string
}

// graph lists the nodes reachable from the inputs, each once, with their UIDs.
func (w *WorkflowView) graph() (*graph, error) {
	g := &graph{uids: make(map[Node]string)}
	used := make(map[string]bool)
	var visit func(n Node, parent string) error
	visit = func(n Node, parent string) error {
		if n == nil {
			return errors.New("workflow graph contains a nil node")
		}
		if _, seen := g.uids[n]; seen {
			return nil
		}
		c := n.base()
		uid := c.id
		switch {
		case uid == "":
			label := strings.Trim(unsafeChars.ReplaceAllString(c.label, "-"), "-")
			if label == "" {
				label = "node"
			}
			if parent != "" {
				label = parent + "." + label
			}
			uid = label
			for i := 2; used[uid]; i++ {
				uid = fmt.Sprintf("%s-%d", label, i)
			}
		case unsafeChars.MatchString(uid):
			return fmt.Errorf("ID %q may only contain letters, digits, '.', '_' and '-'", uid)
		case used[uid]:
			return fmt.Errorf("ID %q is used twice", uid)
		}
		used[uid] = true
		g.uids[n] = uid
		g.nodes = append(g.nodes, n)
		for _, l := range c.links {
			if err := visit(l.to, uid); err != nil {
				return err
			}
		}
		return nil
	}
	for _, n := range w.inputs {
		if err := visit(n, ""); err != nil {
			return nil, err
		}
	}
	return g, nil
}

// layout places each node in a column by its distance from an input. Without
// positions, Alfred's editor draws every node in the same spot.
func (g *graph) layout() map[string]any {
	depth := make(map[Node]int)
	for range g.nodes {
		for _, n := range g.nodes {
			for _, l := range n.base().links {
				depth[l.to] = max(depth[l.to], depth[n]+1)
			}
		}
	}
	rows := make(map[int]int)
	positions := make(map[string]any)
	for _, n := range g.nodes {
		d := depth[n]
		positions[g.uids[n]] = map[string]any{
			"xpos": 50 + 250*d,
			"ypos": 50 + 150*rows[d],
		}
		rows[d]++
	}
	return positions
}

func (w *WorkflowView) plist() (map[string]any, error) {
	if w.bundleID == "" {
		return nil, errors.New("workflow needs a BundleID")
	}
	g, err := w.graph()
	if err != nil {
		return nil, err
	}
	var objects []any
	connections := make(map[string]any)
	for _, n := range g.nodes {
		uid := g.uids[n]
		kind, version, config := n.object(uid)
		objects = append(objects, map[string]any{
			"uid":     uid,
			"type":    kind,
			"version": version,
			"config":  config,
		})
		var links []any
		for _, l := range n.base().links {
			links = append(links, map[string]any{
				"destinationuid":  g.uids[l.to],
				"modifiers":       int(l.mod),
				"modifiersubtext": l.subtitle,
				"vitoclose":       false,
			})
		}
		if links != nil {
			connections[uid] = links
		}
	}
	p := map[string]any{
		"name":        w.name,
		"bundleid":    w.bundleID,
		"version":     w.version,
		"createdby":   w.author,
		"description": w.description,
		"readme":      w.readme,
		"webaddress":  w.website,
		"disabled":    false,
		"objects":     objects,
		"connections": connections,
		"uidata":      g.layout(),
	}
	if w.category != "" {
		p["category"] = w.category
	}
	if w.vars != nil {
		p["variables"] = w.vars
	}
	return p, nil
}

type packagedFile struct {
	name, source string
	data         []byte
	mode         os.FileMode
}

// build writes dir/<name>.alfredworkflow and returns its path.
func (w *WorkflowView) build(dir string) (string, error) {
	p, err := w.plist()
	if err != nil {
		return "", err
	}
	var info bytes.Buffer
	if err := encodePlist(&info, p); err != nil {
		return "", err
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	files := []packagedFile{
		{name: "info.plist", data: info.Bytes(), mode: 0o644},
		{name: binaryName, source: exe, mode: 0o755},
	}
	if w.icon != "" {
		files = append(files, packagedFile{name: "icon.png", source: w.icon, mode: 0o644})
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := strings.Trim(unsafeChars.ReplaceAllString(w.name, "-"), "-")
	if name == "" {
		name = "workflow"
	}
	path := filepath.Join(dir, name+".alfredworkflow")
	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer out.Close()
	archive := zip.NewWriter(out)
	for _, f := range files {
		data := f.data
		if f.source != "" {
			if data, err = os.ReadFile(f.source); err != nil {
				return "", err
			}
		}
		header := &zip.FileHeader{Name: f.name, Method: zip.Deflate, Modified: time.Now()}
		header.SetMode(f.mode)
		entry, err := archive.CreateHeader(header)
		if err != nil {
			return "", err
		}
		if _, err := entry.Write(data); err != nil {
			return "", err
		}
	}
	if err := archive.Close(); err != nil {
		return "", err
	}
	return path, out.Close()
}
