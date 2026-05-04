package color

const (
	Reset        = "\x1b[0m"
	BrightBlack  = "\x1b[90m"
	Red          = "\x1b[31m"
	Green        = "\x1b[32m"
	Yellow       = "\x1b[33m"
	Blue         = "\x1b[34m"
	Magenta      = "\x1b[35m"
	Cyan         = "\x1b[36m"
	White        = "\x1b[37m"
	BrightGreen  = "\x1b[92m"
	BrightYellow = "\x1b[93m"
	BrightRed    = "\x1b[91m"
	BrightCyan   = "\x1b[96m"
	Dim          = "\x1b[2m"
	Bold         = "\x1b[1m"
)

func Wrap(code, s string) string {
	if s == "" {
		return ""
	}
	return code + s + Reset
}
