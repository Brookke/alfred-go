package alfred

import (
	"fmt"
	"strings"
)

// HotkeyNode is a keyboard shortcut that starts the workflow.
type HotkeyNode struct {
	node[*HotkeyNode]
	mods Modifier
	key  string
}

// Hotkey runs the nodes connected with Then when key is pressed with mods
// held, such as Hotkey(Cmd|Alt, "S"). key is a letter, digit or punctuation
// character, or a name such as "Space", "Return", "Up" or "F5". An empty key
// leaves the shortcut unset, to be recorded in Alfred after importing.
func Hotkey(mods Modifier, key string) *HotkeyNode {
	h := &HotkeyNode{mods: mods, key: key}
	h.self, h.label = h, "hotkey"
	return h
}

func (h *HotkeyNode) validate() error {
	if _, ok := keyCode(h.key); !ok {
		return fmt.Errorf("hotkey: unknown key %q", h.key)
	}
	return nil
}

func (h *HotkeyNode) object(string) (string, int, map[string]any) {
	code, _ := keyCode(h.key)
	mods, display := int(h.mods), strings.ToUpper(h.key)
	if h.key == "" {
		mods = 0
	}
	return "alfred.workflow.trigger.hotkey", 2, map[string]any{
		"action": 0, "argument": 0, "focusedappvariable": false, "focusedappvariablename": "",
		"hotkey": code, "hotmod": mods, "hotstring": display,
		"leftcursor": false, "modsmode": 0, "relatedAppsMode": 0,
	}
}

// keyCode is key's macOS virtual key code (US layout). The empty key is 0,
// which Alfred treats as unset.
func keyCode(key string) (int, bool) {
	if key == "" {
		return 0, true
	}
	code, ok := keyCodes[strings.ToLower(key)]
	return code, ok
}

var keyCodes = map[string]int{
	"a": 0, "s": 1, "d": 2, "f": 3, "h": 4, "g": 5, "z": 6, "x": 7, "c": 8, "v": 9,
	"b": 11, "q": 12, "w": 13, "e": 14, "r": 15, "y": 16, "t": 17,
	"1": 18, "2": 19, "3": 20, "4": 21, "6": 22, "5": 23, "=": 24, "9": 25, "7": 26,
	"-": 27, "8": 28, "0": 29, "]": 30, "o": 31, "u": 32, "[": 33, "i": 34, "p": 35,
	"l": 37, "j": 38, "'": 39, "k": 40, ";": 41, `\`: 42, ",": 43, "/": 44, "n": 45,
	"m": 46, ".": 47, "`": 50,
	"return": 36, "tab": 48, "space": 49, "delete": 51, "escape": 53,
	"forwarddelete": 117, "home": 115, "end": 119, "pageup": 116, "pagedown": 121,
	"left": 123, "right": 124, "down": 125, "up": 126,
	"f1": 122, "f2": 120, "f3": 99, "f4": 118, "f5": 96, "f6": 97, "f7": 98, "f8": 100,
	"f9": 101, "f10": 109, "f11": 103, "f12": 111, "f13": 105, "f14": 107, "f15": 113,
	"f16": 106, "f17": 64, "f18": 79, "f19": 80, "f20": 90,
}
