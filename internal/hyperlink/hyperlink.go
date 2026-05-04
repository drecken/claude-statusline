package hyperlink

// Osc8 wraps text in an OSC 8 hyperlink escape so supported terminals render
// it clickable. Uses BEL terminator (0x07) for iTerm2/kitty/wezterm compat.
func Osc8(url, text string) string {
	if url == "" {
		return text
	}
	return "\x1b]8;;" + url + "\x07" + text + "\x1b]8;;\x07"
}
