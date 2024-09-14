package tui

import (
	"fmt"
	"strings"

	"github.com/canonical/lxd/shared"
	tea "github.com/charmbracelet/bubbletea"
)

// Asker represents a text input question asker.
type Asker struct {
	question        string
	defaultAnswer   string
	acceptedAnswers []string

	answer  string
	options []tea.ProgramOption

	cancelled bool
}

func NewAsker() *Asker {
	return &Asker{}
}

func (a *Asker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (a *Asker) View() string {
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

func (a *Asker) Init() tea.Cmd {
	a.cancelled = false
	a.answer = ""

	return nil
}

// AskBoolWarn is the same as AskBool but it appends "! Warning:" to the front of the message.
func (a *Asker) AskBoolWarn(question string, defaultAnswer bool) (bool, error) {
	question = fmt.Sprintf("%s %s: %s", WarningSymbol(), WarningColor("Warning", true), question)
	return a.AskBool(question, defaultAnswer)
}

// AskBool prints the given prompt and accepts either "yes" or "no" as answers. It returns the boolean equivalent.
func (a *Asker) AskBool(question string, defaultAnswer bool) (bool, error) {
	a = &Asker{}
	a.acceptedAnswers = []string{"yes", "no"}
	defaultAnswerStr := "no"
	if defaultAnswer {
		defaultAnswerStr = "yes"
	}

	result, err := a.handleQuestion(question, defaultAnswerStr)
	if err != nil {
		return false, err
	}

	if shared.ValueInSlice(strings.ToLower(result.answer), []string{"yes", "y"}) {
		fmt.Println(a.View())
		return true, nil
	} else if shared.ValueInSlice(strings.ToLower(result.answer), []string{"no", "n"}) {
		fmt.Println(a.View())
		return false, nil
	}

	return false, fmt.Errorf("Response %q must be one of %v", result.answer, result.acceptedAnswers)
}

// AskString prints the given prompt and accepts a string answer. If no answer is provided, it uses the default answer.
func (a *Asker) AskString(question string, defaultAnswer string, validator func(string) error) (string, error) {
	a = &Asker{}
	result, err := a.handleQuestion(question, defaultAnswer)
	if err != nil {
		return "", err
	}

	err = validator(result.answer)
	if err != nil {
		return "", err
	}

	fmt.Println(a.View())

	return result.answer, nil
}

func (a *Asker) handleQuestion(question string, defaultAnswer string) (*Asker, error) {
	a.question = question
	a.defaultAnswer = defaultAnswer

	out, err := tea.NewProgram(a).Run()
	if err != nil {
		return nil, err
	}

	result, ok := out.(*Asker)
	if !ok {
		return nil, fmt.Errorf("Unexpected question result")
	}

	if result.cancelled {
		return nil, fmt.Errorf("Input cancelled")
	}

	if strings.TrimSpace(result.answer) == "" {
		result.answer = result.defaultAnswer
	} else {
		result.answer = strings.TrimSpace(result.answer)
	}

	return result, nil
}
