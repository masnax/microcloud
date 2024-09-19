package main

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/canonical/microcloud/microcloud/cmd/tui"
)

func main2() {
	a := tui.NewInputHandler(os.Stdin, os.Stdout)

	a.AskBool("test q", false)
	a.AskString("test q", "abcd", func(s string) error { return nil })
}

func printMessageWithCursor(message string, cursorCol int) {
	// Clear the screen
	fmt.Print("\x1b[2J")
	// Move cursor to the start of the line
	fmt.Print("\x1b[H")
	// Print the message
	fmt.Print(message)
	// Move the cursor to the specified column (after the message)
	fmt.Printf("\x1b[%dG", len(message)+1)
	// Show the cursor
	fmt.Print("\x1b[?25h")
}

func main() {
	//message := "my message:"
	//printMessageWithCursor(message, len(message)+1)

	if 1 > 0 {
		table := tui.NewTable([]string{"a", "b"}, [][]string{{"a1", "b1"}, {"a2", "b2"}})
		fmt.Println(table)

		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b := bytes.Buffer{}
	for _, line := range []string{"b2", "table:select", "table:done"} {
		_, _ = b.WriteString(fmt.Sprintf("%s\n", line))
	}

	out, _ := os.CreateTemp("", "test-output")
	fmt.Println(out.Name())
	//r := bufio.NewReader(bytes.NewReader(b.Bytes()))
	//asker, _ := tui.PrepareTestAsker(ctx, r, out)

	asker := tui.NewInputHandler(os.Stdin, os.Stdout)

	table := tui.NewSelectableTable([]string{"a", "b"}, [][]string{{"a1", "b1"}, {"a2", "b2"}})
	answers, _ := table.Render(ctx, asker, "title")

	fmt.Println(answers)
}
