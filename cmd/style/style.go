package style

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	// White represents the common white color used throughout the CLI.
	White = "255"

	// DimWhite represents the white color used throughout the CLI.
	DimWhite = "252"

	// Grey represents the common grey color used throughout the CLI.
	Grey = "247"

	// DarkGrey represents the common dark grey color used throughout the CLI.
	DarkGrey = "240"

	// Red represents the common red color used throughout the CLI.
	Red = "197"

	// Green represents the common green color used throughout the CLI.
	Green = "42"

	// Yellow represents the common yellow color used throughout the CLI.
	Yellow = "214"

	// Purple represnts the common purple color used throughout the CLI.
	Purple = "139"

	// Orange represents the common orange color used throughoug the CLI.
	Orange = "208"
)

// SetColor applies the color to the given text.
func SetColor(color string, str string, bold bool) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).SetString(str).Bold(bold).String()
}

// TextColor sets the 8-bit color of the text to 252. Used for basic text.
func TextColor(text string) string {
	return SetColor(White, text, false)
}

// TableBorderColor sets the 8-bit color of the text to 248. Used for table header text.
func TableHeaderColor(text string) string {
	return SetColor(Grey, text, false)
}

// TableBorderColor sets the 8-bit color of the text to 238. Used for table borders.
func TableBorderColor(text string) string {
	return SetColor(DarkGrey, text, false)
}

// SuccessColor sets the 8-bit color of the text to 82. Green color used for happy messages.
func SuccessColor(text string) string {
	return SetColor(Green, text, false)
}

// WarningColor sets the 8-bit color of the text to 214. Yellow color used for warning messages.
func WarningColor(text string) string {
	return SetColor(Yellow, text, false)
}

// ErrorColor sets the 8-bit color of the text to 197. Red color used for error messages.
func ErrorColor(text string) string {
	return SetColor(Red, text, false)
}

// WarningSymbol returns a Yellow colored ! symbol to use for warnings.
func WarningSymbol() string {
	return WarningColor("!")
}

// ErrorSymbol returns a Red colored ⨯ symbol to use for errors.
func ErrorSymbol() string {
	return ErrorColor("⨯")
}

// SuccessSymbol returns a Green colored ✓ symbol to use for success messages.
func SuccessSymbol() string {
	return SuccessColor("✓")
}

// Format represents the data supplied to ColorPrintf. In particular, it takes a color to apply to the text, and the text itself.
type Format struct {
	Color string
	Arg   any
	Bold  bool
}

// ColorPrintf works like fmt.Sprintf except it applies custom coloring to each argument, and the main template body.
func ColorPrintf(template Format, args ...Format) string {
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
		styledArg := lipgloss.NewStyle().Foreground(lipgloss.Color(template.Color)).Bold(template.Bold).SetString(part).String()
		// The styling seems to cause newlines to print extra spaces so trim them.
		styledArg = strings.TrimSpace(styledArg)
		_, _ = builder.WriteString(styledArg)
		if i < len(args) {
			styleTmpl := lipgloss.NewStyle().Foreground(lipgloss.Color(args[i].Color)).Bold(args[i].Bold)
			_, _ = builder.WriteString(styleTmpl.SetString(fmt.Sprintf(directives[i], args[i].Arg)).String())
		}
	}

	return builder.String()
}
