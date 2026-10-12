package alfred

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
)

// encodePlist writes v as an XML property list, the format of info.plist.
func encodePlist(w io.Writer, v any) error {
	var b bytes.Buffer
	b.WriteString(xml.Header)
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0">` + "\n")
	if err := writePlistValue(&b, v, 0); err != nil {
		return err
	}
	b.WriteString("</plist>\n")
	_, err := w.Write(b.Bytes())
	return err
}

func writePlistValue(b *bytes.Buffer, v any, depth int) error {
	indent := strings.Repeat("\t", depth)
	switch v := v.(type) {
	case string:
		b.WriteString(indent + "<string>")
		xml.EscapeText(b, []byte(v))
		b.WriteString("</string>\n")
	case bool:
		if v {
			b.WriteString(indent + "<true/>\n")
		} else {
			b.WriteString(indent + "<false/>\n")
		}
	case int:
		fmt.Fprintf(b, "%s<integer>%d</integer>\n", indent, v)
	case int64:
		fmt.Fprintf(b, "%s<integer>%d</integer>\n", indent, v)
	case float64:
		fmt.Fprintf(b, "%s<real>%g</real>\n", indent, v)
	case []string:
		values := make([]any, len(v))
		for i, s := range v {
			values[i] = s
		}
		return writePlistValue(b, values, depth)
	case []any:
		b.WriteString(indent + "<array>\n")
		for _, value := range v {
			if err := writePlistValue(b, value, depth+1); err != nil {
				return err
			}
		}
		b.WriteString(indent + "</array>\n")
	case map[string]string:
		values := make(map[string]any, len(v))
		for key, s := range v {
			values[key] = s
		}
		return writePlistValue(b, values, depth)
	case map[string]any:
		var keys []string
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		b.WriteString(indent + "<dict>\n")
		for _, key := range keys {
			b.WriteString(indent + "\t<key>")
			xml.EscapeText(b, []byte(key))
			b.WriteString("</key>\n")
			if err := writePlistValue(b, v[key], depth+1); err != nil {
				return err
			}
		}
		b.WriteString(indent + "</dict>\n")
	default:
		return fmt.Errorf("plist: unsupported value %T", v)
	}
	return nil
}
