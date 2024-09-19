//go:build test

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/canonical/microcloud/microcloud/cmd/tui"
)

func setupAsker(ctx context.Context) (*tui.InputHandler, error) {
	noColor := os.Getenv("NO_COLOR")
	if noColor != "" {
		tui.DisableColors()
	}

	useTestConsole := os.Getenv("TEST_CONSOLE")
	if useTestConsole != "1" {
		return tui.NewInputHandler(os.Stdin, os.Stdout), nil
	}

	fmt.Fprintf(os.Stderr, "%s\n\n", `
  Detected 'TEST_CONSOLE=1', MicroCloud CLI is in testing mode. Terminal interactivity is disabled.

  Interactive microcloud commands will read text instructions by line:

cat << EOF | microcloud init
select                # selects an element in the table
select-all            # selects all elements in the table
select-none           # de-selects all elements in the table
up                    # move up in the table
down                  # move down in the table
wait <time.Duration>  # waits before the next instruction
expect <count>        # waits until exactly <count> peers are available, and errors out if more are found
---                   # confirms the table selection and exits the table
clear                 # clears the last line
anything else         # will be treated as a raw string. This is useful for filtering a table and text entry
EOF`)

	filePath := "/var/snap/microcloud/common/test-output"
	_ = os.Remove(filePath)
	out, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
	if err != nil {
		return nil, err
	}

	asker, err := tui.PrepareTestAsker(ctx, os.Stdin, out)
	if err != nil {
		return nil, err
	}

	return asker, nil
}
