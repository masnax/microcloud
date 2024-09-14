package tui

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	// White represents the common bright white color used throughout the CLI.
	BrightWhite = "15"

	// BrightBlack represents the common bright black color used throughout the CLI.
	BrightBlack = "8"

	// Grey represents the common white color used throughout the CLI.
	White = "7"

	// Red represents the common red color used throughout the CLI.
	Red = "9"

	// Green represents the common green color used throughout the CLI.
	Green = "10"

	// Yellow represents the common yellow color used throughout the CLI.
	Yellow = "11"

	Cyan = "12"
)

// SetColor applies the color to the given text.
func SetColor(color string, str string, bold bool) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).SetString(str).Bold(bold).String()
}

//// TextColor sets the 8-bit color of the text to 252. Used for basic text.
//func TextColor(text string) string {
//	return SetColor(White, text, false)
//}
//
//// TableBorderColor sets the 8-bit color of the text to 248. Used for table header text.
//func TableHeaderColor(text string) string {
//	return SetColor(Grey, text, false)
//}
//
//// TableBorderColor sets the 8-bit color of the text to 238. Used for table borders.
//func TableBorderColor(text string) string {
//	return SetColor(DarkGrey, text, false)
//}

// SuccessColor sets the 8-bit color of the text to 82. Green color used for happy messages.
func SuccessColor(text string, bold bool) string {
	return SetColor(Green, text, bold)
}

// WarningColor sets the 8-bit color of the text to 214. Yellow color used for warning messages.
func WarningColor(text string, bold bool) string {
	return SetColor(Yellow, text, bold)
}

// ErrorColor sets the 8-bit color of the text to 197. Red color used for error messages.
func ErrorColor(text string, bold bool) string {
	return SetColor(Red, text, bold)
}

// WarningSymbol returns a Yellow colored ! symbol to use for warnings.
func WarningSymbol() string {
	return WarningColor("!", true)
}

// ErrorSymbol returns a Red colored ⨯ symbol to use for errors.
func ErrorSymbol() string {
	return ErrorColor("⨯", true)
}

// SuccessSymbol returns a Green colored ✓ symbol to use for success messages.
func SuccessSymbol() string {
	return SuccessColor("✓", true)
}

// ColorErr is an io.Writer wrapper around os.Stderr that prints the error with colored text.
type ColorErr struct{}

// Write colors the given error red and sends it to os.Stderr.
func (*ColorErr) Write(p []byte) (n int, err error) {
	return os.Stderr.WriteString(ErrorColor(strings.TrimSpace(string(p)), false) + "\n")
}

// Fmt represents the data supplied to ColorPrintf. In particular, it takes a color to apply to the text, and the text itself.
type Fmt struct {
	Color string
	Arg   any
	Bold  bool
}

// Printf works like fmt.Sprintf except it applies custom coloring to each argument, and the main template body.
func Printf(template Fmt, args ...Fmt) string {
	var builder strings.Builder

	// Match format directives.
	re := regexp.MustCompile(`%[+]?[vTtbcdoqxXUeEfFgGsqp]`)

	// Split the string on format directives
	parts := re.Split(template.Arg.(string), -1)
	directives := re.FindAllString(template.Arg.(string), -1)

	if len(directives) != len(args) {
		return fmt.Sprintf("Invalid format (expected %d args but found %d)", len(directives), len(args))
	}

	for i, part := range parts {
		styledArg := ""

		// The styling seems to cause newlines to print extra spaces so trim them.
		preNL, ok := strings.CutSuffix(part, "\n ")
		postNL := ""
		if ok {
			postNL = "\n "
		}

		styledTmpl := lipgloss.NewStyle().Bold(template.Bold).SetString(preNL)
		styledTmpl = styledTmpl.Foreground(lipgloss.Color(template.Color))
		styledArg += styledTmpl.String()

		_, _ = builder.WriteString(styledArg + postNL)
		if i < len(args) {
			styledTmpl := lipgloss.NewStyle().Bold(args[i].Bold)
			styledTmpl = styledTmpl.Foreground(lipgloss.Color(args[i].Color))
			_, _ = builder.WriteString(styledTmpl.SetString(fmt.Sprintf(directives[i], args[i].Arg)).String())
		}
	}

	return builder.String()
}
