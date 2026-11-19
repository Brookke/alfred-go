package ui

// ViewComponent is a container whose layout places its components.
type ViewComponent struct{ component[*ViewComponent] }

func newView(layout string, children []Component) *ViewComponent {
	v := newComponent("view", &ViewComponent{}, func() *ViewComponent {
		p := &ViewComponent{}
		p.self = p
		return p
	})
	v.set("layout", layout)
	return v.Add(children...)
}

// Border lays children out by their Region: top and bottom take the full
// width, left and right the remaining height, and centre the rest.
func Border(children ...Component) *ViewComponent { return newView("border", children) }

// Stack lays children over each other, each filling the view.
func Stack(children ...Component) *ViewComponent { return newView("stack", children) }

// HStack lays children out in a row.
func HStack(children ...Component) *ViewComponent { return newView("inline", children) }

// VStack lays children out in a column.
func VStack(children ...Component) *ViewComponent {
	return newView("inline", children).Set("direction", "vertical")
}

// Grid lays children out in equal cells, cols wide and rows high.
func Grid(cols, rows int, children ...Component) *ViewComponent {
	return newView("grid", children).Set("cols", cols).Set("rows", rows)
}

// Flex lays children out in a grid where each places itself with Grid.
func Flex(cols, rows int, children ...Component) *ViewComponent {
	return newView("flex", children).Set("cols", cols).Set("rows", rows)
}

// Form lays children out in label and field pairs.
func Form(children ...Component) *ViewComponent { return newView("form", children) }

// Add appends children.
func (v *ViewComponent) Add(children ...Component) *ViewComponent {
	if v.target != nil {
		panic("ui: a patch can't add components; use Insert")
	}
	v.children = append(v.children, children...)
	return v
}

func (v *ViewComponent) Padding(padding int) *ViewComponent { return v.Set("padding", padding) }
func (v *ViewComponent) Spacing(spacing int) *ViewComponent { return v.Set("spacing", spacing) }
func (v *ViewComponent) CornerRadius(radius int) *ViewComponent {
	return v.Set("cornerradius", radius)
}

// Clip clips the view's contents to its bounds, such as rounded corners.
func (v *ViewComponent) Clip() *ViewComponent { return v.Set("cliptobounds", true) }

// Border draws a border inset in the view; width 0 removes it.
func (v *ViewComponent) Border(width int, colour Colour) *ViewComponent {
	return v.Set("border", map[string]any{"width": width, "colour": colour})
}

// Shortcut runs actions when key is pressed while focus is in this view,
// such as "⌘P", "↩" or "↑".
func (v *ViewComponent) Shortcut(key string, actions ...Action) *ViewComponent {
	if v.target != nil {
		panic("ui: shortcuts can't be patched")
	}
	v.shortcuts = append(v.shortcuts, shortcut{key: key, actions: actions})
	return v
}

// LabelComponent is a line or block of read-only text.
type LabelComponent struct{ component[*LabelComponent] }

func Label() *LabelComponent {
	return newComponent("label", &LabelComponent{}, func() *LabelComponent {
		p := &LabelComponent{}
		p.self = p
		return p
	})
}

func (l *LabelComponent) Text(text string) *LabelComponent     { return l.Set("text", text) }
func (l *LabelComponent) Colour(colour Colour) *LabelComponent { return l.Set("colour", colour) }
func (l *LabelComponent) Font(font Font) *LabelComponent       { return l.Set("font", font) }
func (l *LabelComponent) Align(align Align) *LabelComponent    { return l.Set("textalign", string(align)) }
func (l *LabelComponent) LineBreak(lb LineBreak) *LabelComponent {
	return l.Set("linebreak", string(lb))
}
func (l *LabelComponent) Selectable() *LabelComponent { return l.Set("selectable", true) }

// Align is how text is aligned.
type Align string

const (
	AlignLeft    Align = "left"
	AlignRight   Align = "right"
	AlignCentre  Align = "centre"
	AlignJustify Align = "justify"
	AlignNatural Align = "natural"
)

// LineBreak is how text wraps or truncates.
type LineBreak string

const (
	TruncateTail   LineBreak = "truncatetail"
	TruncateHead   LineBreak = "truncatehead"
	TruncateMiddle LineBreak = "truncatemiddle"
	WordWrap       LineBreak = "wordwrap"
	CharWrap       LineBreak = "charwrap"
	Clip           LineBreak = "clip"
)

// TextFieldComponent is a single line of editable text.
type TextFieldComponent struct{ component[*TextFieldComponent] }

func TextField() *TextFieldComponent {
	return newComponent("textfield", &TextFieldComponent{}, func() *TextFieldComponent {
		p := &TextFieldComponent{}
		p.self = p
		return p
	})
}

func (t *TextFieldComponent) Text(text string) *TextFieldComponent { return t.Set("text", text) }
func (t *TextFieldComponent) Placeholder(text string) *TextFieldComponent {
	return t.Set("placeholder", text)
}
func (t *TextFieldComponent) Font(font Font) *TextFieldComponent { return t.Set("font", font) }
func (t *TextFieldComponent) Bordered(bordered bool) *TextFieldComponent {
	return t.Set("bordered", bordered)
}

// TextValue is a placeholder for the field's text when an action runs.
func (t *TextFieldComponent) TextValue() string { return t.Get("text") }

// TextAreaComponent is a block of text, editable or not.
type TextAreaComponent struct{ component[*TextAreaComponent] }

func TextArea() *TextAreaComponent {
	return newComponent("textarea", &TextAreaComponent{}, func() *TextAreaComponent {
		p := &TextAreaComponent{}
		p.self = p
		return p
	})
}

func (t *TextAreaComponent) Text(text string) *TextAreaComponent { return t.Set("text", text) }
func (t *TextAreaComponent) Font(font Font) *TextAreaComponent   { return t.Set("font", font) }
func (t *TextAreaComponent) Editable(editable bool) *TextAreaComponent {
	return t.Set("editable", editable)
}

// TextValue is a placeholder for the area's text when an action runs.
func (t *TextAreaComponent) TextValue() string { return t.Get("text") }

// ButtonComponent is a push button. Its Actioned event fires when clicked.
type ButtonComponent struct{ component[*ButtonComponent] }

func Button(text string) *ButtonComponent {
	b := newComponent("button", &ButtonComponent{}, func() *ButtonComponent {
		p := &ButtonComponent{}
		p.self = p
		return p
	})
	return b.Text(text)
}

func (b *ButtonComponent) Text(text string) *ButtonComponent  { return b.Set("text", text) }
func (b *ButtonComponent) Image(image Image) *ButtonComponent { return b.Set("image", image) }
func (b *ButtonComponent) Bordered(bordered bool) *ButtonComponent {
	return b.Set("bordered", bordered)
}

// Shortcut presses the button with a key combination such as "⌘S".
func (b *ButtonComponent) Shortcut(key string) *ButtonComponent { return b.Set("shortcut", key) }

// PickerComponent is a popup menu of items.
type PickerComponent struct{ component[*PickerComponent] }

func Picker(items ...PickerItem) *PickerComponent {
	p := newComponent("picker", &PickerComponent{}, func() *PickerComponent {
		p := &PickerComponent{}
		p.self = p
		return p
	})
	return p.Items(items...)
}

// PickerItem is one item in a Picker.
type PickerItem map[string]any

// Item is a picker item showing text, whose value is value.
func Item(text, value string) PickerItem { return PickerItem{"text": text, "value": value} }

// Divider separates picker items.
func Divider() PickerItem { return PickerItem{"type": "separator"} }

func (p *PickerComponent) Items(items ...PickerItem) *PickerComponent {
	if items == nil {
		items = []PickerItem{}
	}
	return p.Set("items", items)
}

// Select selects the item with value.
func (p *PickerComponent) Select(value string) *PickerComponent { return p.Set("selected", value) }

// SelectedValue is a placeholder for the selected item's value when an action runs.
func (p *PickerComponent) SelectedValue() string { return p.Get("selected.value") }

// ImageViewComponent shows an image.
type ImageViewComponent struct{ component[*ImageViewComponent] }

func ImageView() *ImageViewComponent {
	return newComponent("imageview", &ImageViewComponent{}, func() *ImageViewComponent {
		p := &ImageViewComponent{}
		p.self = p
		return p
	})
}

func (i *ImageViewComponent) Image(image Image) *ImageViewComponent { return i.Set("image", image) }
func (i *ImageViewComponent) Resize(mode Resize) *ImageViewComponent {
	return i.Set("resize", string(mode))
}

// Tint colours symbols and other template images.
func (i *ImageViewComponent) Tint(colour Colour) *ImageViewComponent {
	return i.Set("contenttintcolour", colour)
}
func (i *ImageViewComponent) CornerRadius(radius int) *ImageViewComponent {
	return i.Set("cornerradius", radius)
}

// Resize is how an image fits its view.
type Resize string

const (
	ResizeFit     Resize = "fit"
	ResizeFill    Resize = "fill"
	ResizeStretch Resize = "stretch"
	ResizeCentre  Resize = "centre"
)

// Raw is any other component, by its Alfred type such as "checkbox", set up
// with Set.
func Raw(kind string) *RawComponent {
	return newComponent(kind, &RawComponent{}, func() *RawComponent {
		p := &RawComponent{}
		p.self = p
		return p
	})
}

// RawComponent is a component this package doesn't model.
type RawComponent struct{ component[*RawComponent] }
