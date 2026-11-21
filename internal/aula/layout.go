package aula

// LayoutKey is one key in the physical layout, used by the TUI and the API.
type LayoutKey struct {
	Name  string
	Label string
	Width int // Relative width in grid units (1 = normal key).
}

// KeyLayout is the F108 Pro's 108-key ANSI layout, row by row.
var KeyLayout = [][]LayoutKey{
	{
		{"esc", "Esc", 1},
		{"f1", "F1", 1}, {"f2", "F2", 1}, {"f3", "F3", 1}, {"f4", "F4", 1},
		{"f5", "F5", 1}, {"f6", "F6", 1}, {"f7", "F7", 1}, {"f8", "F8", 1},
		{"f9", "F9", 1}, {"f10", "F10", 1}, {"f11", "F11", 1}, {"f12", "F12", 1},
		{"printscreen", "PrtSc", 1}, {"scrolllock", "ScrLk", 1}, {"pause", "Pause", 1},
	},
	{
		{"grave", "`", 1},
		{"1", "1", 1}, {"2", "2", 1}, {"3", "3", 1}, {"4", "4", 1}, {"5", "5", 1},
		{"6", "6", 1}, {"7", "7", 1}, {"8", "8", 1}, {"9", "9", 1}, {"0", "0", 1},
		{"minus", "-", 1}, {"equal", "=", 1}, {"backspace", "Bksp", 2},
		{"insert", "Ins", 1}, {"home", "Home", 1}, {"pageup", "PgUp", 1},
		{"numlock", "NumLk", 1}, {"numslash", "/", 1}, {"numstar", "*", 1}, {"numminus", "-", 1},
	},
	{
		{"tab", "Tab", 2},
		{"q", "Q", 1}, {"w", "W", 1}, {"e", "E", 1}, {"r", "R", 1}, {"t", "T", 1},
		{"y", "Y", 1}, {"u", "U", 1}, {"i", "I", 1}, {"o", "O", 1}, {"p", "P", 1},
		{"lbracket", "[", 1}, {"rbracket", "]", 1}, {"backslash", "\\", 1},
		{"delete", "Del", 1}, {"end", "End", 1}, {"pagedown", "PgDn", 1},
		{"num7", "7", 1}, {"num8", "8", 1}, {"num9", "9", 1}, {"numplus", "+", 1},
	},
	{
		{"capslock", "Caps", 2},
		{"a", "A", 1}, {"s", "S", 1}, {"d", "D", 1}, {"f", "F", 1}, {"g", "G", 1},
		{"h", "H", 1}, {"j", "J", 1}, {"k", "K", 1}, {"l", "L", 1},
		{"semicolon", ";", 1}, {"quote", "'", 1}, {"enter", "Enter", 2},
		{"num4", "4", 1}, {"num5", "5", 1}, {"num6", "6", 1},
	},
	{
		{"lshift", "Shift", 3},
		{"z", "Z", 1}, {"x", "X", 1}, {"c", "C", 1}, {"v", "V", 1},
		{"b", "B", 1}, {"n", "N", 1}, {"m", "M", 1},
		{"comma", ",", 1}, {"dot", ".", 1}, {"slash", "/", 1}, {"rshift", "Shift", 2},
		{"up", "Up", 1},
		{"num1", "1", 1}, {"num2", "2", 1}, {"num3", "3", 1}, {"numenter", "Ent", 1},
	},
	{
		{"lctrl", "Ctrl", 2}, {"lwin", "Win", 1}, {"lalt", "Alt", 1},
		{"space", "Space", 7},
		{"ralt", "Alt", 1}, {"fn", "Fn", 1}, {"menu", "Menu", 1}, {"rctrl", "Ctrl", 1},
		{"left", "Left", 1}, {"down", "Down", 1}, {"right", "Right", 1},
		{"num0", "0", 2}, {"numdot", ".", 1},
	},
}
