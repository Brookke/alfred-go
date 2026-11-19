package ui

// DataViewComponent shows rows of data, each rendered by its Cell, as a list
// or a grid. Set the rows with Rows, at build time or from a script:
//
//	ui.Update(results.Patch().Rows(
//		ui.Row("a", title.Patch().Text("Apple")),
//		ui.Row("b", title.Patch().Text("Banana")),
//	))
type DataViewComponent struct{ component[*DataViewComponent] }

// DataView shows rows, each in a cell set with Cell.
func DataView() *DataViewComponent {
	d := newComponent("dataview", &DataViewComponent{}, func() *DataViewComponent {
		p := &DataViewComponent{}
		p.self = p
		return p
	})
	return d.Rows()
}

// Cell sets how each row is rendered.
func (d *DataViewComponent) Cell(cell *CellSpec) *DataViewComponent {
	if d.target != nil {
		panic("ui: a data view's cell can't be patched")
	}
	d.cell = cell.root
	return d.Set("cell", cell)
}

// Rows sets the rows shown, in order.
func (d *DataViewComponent) Rows(rows ...RowData) *DataViewComponent {
	if rows == nil {
		rows = []RowData{}
	}
	return d.Set("data", map[string]any{"type": "basic", "src": rows})
}

// Cols lays rows out in a grid this many columns wide.
func (d *DataViewComponent) Cols(cols int) *DataViewComponent       { return d.Set("cols", cols) }
func (d *DataViewComponent) Spacing(spacing int) *DataViewComponent { return d.Set("spacing", spacing) }
func (d *DataViewComponent) Padding(padding int) *DataViewComponent { return d.Set("padding", padding) }

// Placeholder is shown when there are no rows.
func (d *DataViewComponent) Placeholder(text string) *DataViewComponent {
	return d.Set("placeholder", text)
}

// WrapSelection moves the selection from the last row back to the first.
func (d *DataViewComponent) WrapSelection() *DataViewComponent {
	return d.selection("wrap", "vertical")
}

// SelectOnHover selects the row under the mouse, which fires Changed.
func (d *DataViewComponent) SelectOnHover() *DataViewComponent { return d.selection("hover", true) }

// AllowNoSelection lets nothing be selected; Changed then passes null.
func (d *DataViewComponent) AllowNoSelection() *DataViewComponent {
	return d.selection("allowempty", true)
}

func (d *DataViewComponent) selection(key string, value any) *DataViewComponent {
	behaviour, _ := d.props["behaviour"].(map[string]any)
	if behaviour == nil {
		behaviour = map[string]any{}
	}
	selection, _ := behaviour["selection"].(map[string]any)
	if selection == nil {
		selection = map[string]any{}
	}
	selection[key] = value
	behaviour["selection"] = selection
	return d.Set("behaviour", behaviour)
}

// SelectedUID is a placeholder for the selected row's UID when an action runs.
func (d *DataViewComponent) SelectedUID() string { return d.Get("selected.uid") }

// Selected is a placeholder for a key of the selected row, set with RowData.With.
func (d *DataViewComponent) Selected(key string) string { return d.Get("selected." + key) }

// Select selects the row with uid. In a patch alongside Rows, it selects
// within the new rows.
func (d *DataViewComponent) Select(uid string) *DataViewComponent {
	data, _ := d.props["data"].(map[string]any)
	if data == nil {
		data = map[string]any{}
	} else {
		clone := map[string]any{}
		for k, v := range data {
			clone[k] = v
		}
		data = clone
	}
	data["select"] = uid
	return d.Set("data", data)
}

// Functions the data view runs with Call.
const (
	SelectUp       = "selectup"
	SelectDown     = "selectdown"
	SelectLeft     = "selectleft"
	SelectRight    = "selectright"
	SelectFirst    = "selectfirst"
	SelectLast     = "selectlast"
	SelectNext     = "selectnext"
	SelectPrevious = "selectprevious"
	SelectNone     = "selectnone"
	ScrollTop      = "scrolltop"
)

// CellSpec is how a DataView renders each row.
type CellSpec struct {
	root           Component
	height         int
	init, selected []Component
}

// Cell renders each row with root. Rows fill it in with patches to root's
// components.
func Cell(root Component) *CellSpec { return &CellSpec{root: root} }

// Height is each cell's height, 64 by default.
func (c *CellSpec) Height(height int) *CellSpec {
	c.height = height
	return c
}

// Init resets a reused cell before a row is shown in it. Set here anything
// that some rows patch and others don't.
func (c *CellSpec) Init(patches ...Component) *CellSpec {
	c.init = append(c.init, patches...)
	return c
}

// Selected styles the selected row's cell.
func (c *CellSpec) Selected(patches ...Component) *CellSpec {
	c.selected = append(c.selected, patches...)
	return c
}

func (c *CellSpec) MarshalJSON() ([]byte, error) {
	m := map[string]any{"root": c.root.base().json()}
	if c.height > 0 {
		m["height"] = c.height
	}
	if c.init != nil {
		init := map[string]any{}
		flat(init, c.init)
		m["init"] = init
	}
	if c.selected != nil {
		selected := map[string]any{}
		flat(selected, c.selected)
		m["selected"] = selected
	}
	return jsonMarshal(m)
}

// RowData is one row of a DataView.
type RowData map[string]any

// Row is a row with uid, filled in by patches to the cell's components.
func Row(uid string, patches ...Component) RowData {
	r := RowData{"uid": uid}
	flat(r, patches)
	return r
}

// With adds a value to the row for actions to read, with DataViewComponent.Selected.
func (r RowData) With(key string, value any) RowData {
	r[key] = value
	return r
}
