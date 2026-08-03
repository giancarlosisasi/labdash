package keymap

import "strings"

// published is how a key is written for a person: on /keys/, in `?`, and in the
// footer. The map is the keys whose terminal name and printed name differ.
var published = map[string]string{
	"up": "↑", "down": "↓", "left": "←", "right": "→",
	"enter": "⏎", "esc": "Esc", "tab": "Tab", "shift+tab": "⇧Tab",
	"pgup": "PgUp", "pgdown": "PgDn", "home": "Home", "end": "End",
	"space": "Space",
}

// ascii is the same set below 0x7F, at the same width.
//
// The arrows are one cell in both modes on purpose: a key label that grows from
// one cell to four in ASCII mode shifts everything drawn beside it, and these
// four are drawn beside the vim keys they pair with. The word next to them
// carries the meaning either way.
var ascii = map[string]string{
	"up": "^", "down": "v", "left": "<", "right": ">",
	"enter": "Enter", "esc": "Esc", "tab": "Tab", "shift+tab": "Shift+Tab",
	"pgup": "PgUp", "pgdown": "PgDn", "home": "Home", "end": "End",
	"space": "Space",
}

// Label writes one key the way it is published.
func Label(key string) string { return label(key, published) }

// LabelASCII writes one key with no byte above 0x7F.
func LabelASCII(key string) string { return label(key, ascii) }

func label(key string, names map[string]string) string {
	if name, ok := names[key]; ok {
		return name
	}
	if rest, isChord := strings.CutPrefix(key, "ctrl+"); isChord {
		return "Ctrl+" + strings.ToUpper(rest)
	}
	return key
}

// Labels writes a binding and its alternates.
func (e Entry) Labels() []string { return mapKeys(e.Keys(), Label) }

// LabelsASCII is Labels below 0x7F.
func (e Entry) LabelsASCII() []string { return mapKeys(e.Keys(), LabelASCII) }

func mapKeys(keys []string, f func(string) string) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = f(k)
	}
	return out
}
