package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/canonical/microcloud/microcloud/cmd/style"
)

func main() {
	if 1 > 0 {
		x := "a"
		x = x[:len(x)-1]

		a := style.NewAsker()

		y, err := a.AskBool("test", "yes")
		fmt.Println(y, err)

		return
	}

	h := []string{"Name", "Address", "Fingerprint"}
	r := [][]string{
		{"a0", "10.0.0.101", "dflkasdfj"},
		{"a1", "10.0.0.102", "fadfjkasd"},
		{"a2", "10.0.0.103", "xsfalsdfk"},
		{"a3", "10.0.0.104", "asdsdfklj"},
		{"a4", "10.0.0.105", "aargarfav"},
		{"a5", "10.0.0.106", "x4554g45g"},
		{"a6", "10.0.0.107", "dklfj4343"},
		{"a7", "10.0.0.108", "avsdvasvv"},
		{"a8", "10.0.0.109", "6h56h5656"},
	}

	table := style.NewSelectableTable(h, r)

	go func() {
		time.Sleep(2 * time.Second)

		for i := 0; i < 5; i++ {
			go func() {
				time.Sleep(time.Duration(rand.IntN(2)) * time.Second)
				//table.SendUpdate(style.Insert{fmt.Sprintf("x%d", i), fmt.Sprintf("20.0.0.10%d", i), fmt.Sprintf("%dv24z0j2c", i)})
				//table.SendUpdate(style.Remove(0))

				//				table.SendUpdate(style.Disable(2))
			}()
		}
	}()

	result, err := table.Render(context.Background())
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(result)
	}

}

//	if key.String() == "!" {
//		name := fmt.Sprintf("x%d", s.count)
//		addr := fmt.Sprintf("20.0.0.1%02d", s.count)
//		fingerprint := fmt.Sprintf("x%dx%dx%dx", s.count)
//		s.count++
//		update := insertEntryUpdate{name, addr, fingerprint}
//		s.p.Send(update)

//		return s, nil
//	}

//	if key.String() == "@" {
//		update := removeEntryUpdate(rand.IntN(len(s.rawRows)))
//		s.p.Send(update)

//		return s, nil
//	}

//	if key.String() == "#" {
//		update := disableEntryUpdate(rand.IntN(len(s.rawRows)))
//		s.p.Send(update)

//		return s, nil
//	}

//	if key.String() == "$" {
//		update := enableEntryUpdate(rand.IntN(len(s.rawRows)))
//		s.p.Send(update)

//		return s, nil
//	}
