package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// asker represents a text input question asker.
type asker struct {
	// Asker needs to embed the type for os.Stdout so that we can change how the bubbletea Renderer places the cursor.
	*os.File
	windowWidth int

	cancelled       bool
	answer          string
	question        string
	defaultAnswer   string
	acceptedAnswers []string
}

func (a *asker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Grab the dimensions of the terminal window to properly draw the cursor.
	windowMsg, ok := msg.(tea.WindowSizeMsg)
	if ok {
		a.windowWidth = windowMsg.Width
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return a, nil
	}

	if key.Type == tea.KeyEnter {
		return a, tea.Quit
	}

	if key.Type == tea.KeyBackspace {
		if len(a.answer) > 0 {
			a.answer = a.answer[:len(a.answer)-1]
		}

		return a, nil
	}

	if key.Type == tea.KeyDelete {
		if len(a.answer) > 0 {
			a.answer = a.answer[1:len(a.answer)]
		}

		return a, nil
	}

	if key.Type == tea.KeyCtrlJ {
		return a, tea.Quit
	}

	// Ignore key events
	if key.Type == tea.KeyLeft || key.Type == tea.KeyRight || key.Type == tea.KeyUp || key.Type == tea.KeyDown {
		return a, nil
	}

	if key.Type == tea.KeySpace {
		a.answer += " "
		return a, nil
	}

	if key.Type != tea.KeyRunes {
		a.cancelled = true
		a.answer = ""
		return a, tea.Quit
	}

	for _, rune := range key.Runes {
		a.answer += string(rune)
	}

	return a, nil
}

func (a *asker) View() string {
	var acceptedAnswers string
	if len(a.acceptedAnswers) > 0 {
		acceptedAnswers = Printf(Fmt{Arg: " (%s)"}, Fmt{Arg: strings.Join(a.acceptedAnswers, "/"), Bold: true})
	}

	var defaultAnswer string
	if a.defaultAnswer != "" {
		defaultAnswer = fmt.Sprintf("default=%s", a.defaultAnswer)
		defaultAnswer = Printf(Fmt{Arg: " [%s]"}, Fmt{Arg: defaultAnswer, Bold: true})
	}

	answer := WarningColor(strings.TrimSpace(a.answer), true)

	return fmt.Sprintf("%s%s%s: %s", a.question, acceptedAnswers, defaultAnswer, answer)
}

func (a *asker) Init() tea.Cmd {
	a.cancelled = false
	a.answer = ""

	return tea.ShowCursor
}

// Write changes the cursor position of the line so that it appears in the proper spot at the end of the line.
// The sequence is set at the end of the string by default, causing the string to render the cursor in the first cell.
// Instead, by appending it to the front of the string, the cursor will reset the previously rendered line only.
func (a *asker) Write(b []byte) (int, error) {
	str := string(b)
	str, ok := strings.CutSuffix(str, ansi.CursorLeft(a.windowWidth))
	if ok {
		str = ansi.CursorLeft(a.windowWidth) + str
	}

	return a.File.Write([]byte(str))
}
