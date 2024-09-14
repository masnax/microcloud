package main

import (
	"fmt"

	"github.com/canonical/microcloud/microcloud/cmd/tui"
)

func main() {
	helpEnter := tui.Fmt{Color: tui.White, Arg: "enter", Bold: true}
	helpSpace := tui.Fmt{Color: tui.White, Arg: "space", Bold: true}
	helpType := tui.Fmt{Color: tui.White, Arg: "type", Bold: true}

	helpTmpl := tui.Fmt{Color: tui.White, Arg: " %s to select; %s to confirm; %s to filter results.\n %sto select none."}
	help := tui.Printf(helpTmpl, helpSpace, helpEnter, helpType, helpType)

	fmt.Printf("[%v]", help)

}
