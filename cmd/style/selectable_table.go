package style

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// RemoveMsg is a table notification that returns a row index of the table to remove.
type RemoveMsg int

// DisableMsg is a table notification that returns a row index of the table to disable.
type DisableMsg int

// EnableMsg is a table notification that returns a row index of the table to enable, if it was disabled.
type EnableMsg int

// InsertMsg is a table notification that returns a set of row data to add to the table.
type InsertMsg []string

// defaultTableSize is the maximum number of rows that will be displayed at once.
const defaultTableSize = 5

type selectableTable struct {
	p *tea.Program
	t *table.Table

	// mutex to synchronize update calls that change the rows of the table.
	//updateMu sync.Mutex

	// size is the number of rows that will be displayed at once.
	size int

	// table headers.
	header []string

	// raw row text, without any formatting.
	rawRows [][]string

	// displayed rows with style formatting.
	formatRows [][]string

	// filterMap is a mapping of displayed indexes to absolute indexes in the rawRows list.
	filterMap map[int]int

	// row index for the current cursor position in the whole table.
	currentRow int

	// startIndex is the start index of the current viewport.
	startIndex int

	// selected rows.
	activeRows map[int]bool

	// restricted roes that cannot be selected.
	disabledRows map[int]bool

	// filter is the string used to filter the table results.
	filter string

	// done records whether the table has completed its selection, and thus should erase itself.
	done bool
}

// SummarizeResult formats the result string and args with the standard style for table result summaries.
func SummarizeResult(tmpl string, args ...any) string {
	fmtArgs := []Format{}
	for _, arg := range args {
		fmtArgs = append(fmtArgs, Format{Arg: arg, Color: Orange, Bold: true})
	}

	return ColorPrintf(Format{Arg: fmt.Sprintf(" %s", tmpl), Color: Grey}, fmtArgs...)
}

// NewSelectableTable takes a slice of structs and adds table rows for them.
func NewSelectableTable(header []string, rows [][]string) *selectableTable {
	s := &selectableTable{
		header:  header,
		rawRows: rows,
		size:    defaultTableSize,
	}

	// Lock the table initially so that any updates that run before we are done rendering will be queued.
	//s.updateMu.Lock()

	return s
}

// Render is a blocking function that renders the table until the user exits out, and then returns the selection from the supplied rows.
func (s *selectableTable) Render(ctx context.Context) ([]map[string]string, error) {
	s.done = false
	s.p = tea.NewProgram(s, tea.WithContext(ctx))
	//s.updateMu.Unlock()
	result, err := s.p.Run()
	if err != nil {
		return nil, fmt.Errorf("Failed to render table: %w", err)
	}

	table, ok := result.(*selectableTable)
	if !ok {
		return nil, fmt.Errorf("Unexpected table type")
	}

	resultMap := make([]map[string]string, 0, len(table.rawRows))
	for i := range table.activeRows {
		if table.disabledRows[i] {
			continue
		}

		row := table.rawRows[i]
		rowMap := make(map[string]string, len(row))
		for j, col := range row {
			rowMap[table.header[j]] = col
		}

		resultMap = append(resultMap, rowMap)
	}

	return resultMap, nil
}

func (s *selectableTable) ReplaceRows(newRowMap []map[string]string) error {
	if !s.done {
		return fmt.Errorf("Cannot replace table rows while the table is active")
	}

	newRows := make([][]string, len(newRowMap))
	for i := range newRowMap {
		newRows[i] = make([]string, len(s.header))
		for j, h := range s.header {
			newRows[i][j] = newRowMap[i][h]
		}
	}

	s.rawRows = newRows

	return nil
}

// SendUpdate sends a synchronous update to the table.
func (s *selectableTable) SendUpdate(msg tea.Msg) {
	//s.updateMu.Lock()
	//defer s.updateMu.Unlock()
	if s.p != nil {
		s.p.Send(msg)
	}

}

// Update handles table updates.
func (s *selectableTable) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	//s.updateMu.Lock()
	//defer s.updateMu.Unlock()

	switch msg.(type) {
	case tea.KeyMsg:
		return s.handleKeyEvent(msg.(tea.KeyMsg))
	case InsertMsg:
		return s.handleInsertEvent(msg.(InsertMsg))
	case RemoveMsg:
		return s.handleRemoveEvent(msg.(RemoveMsg))
	case EnableMsg:
		return s.handleEnableEvent(msg.(EnableMsg))
	case DisableMsg:
		return s.handleDisableEvent(msg.(DisableMsg))
	}

	return s, nil
}

// Init initializes a new selectable table.
func (s *selectableTable) Init() tea.Cmd {
	s.activeRows = make(map[int]bool)
	s.disabledRows = make(map[int]bool)
	s.formatRows = make([][]string, 0, len(s.rawRows))
	s.filterMap = make(map[int]int, len(s.rawRows))

	s.currentRow = 0
	s.startIndex = 0
	s.filter = ""

	for i, row := range s.rawRows {
		s.filterMap[i] = i
		col := make([]string, 0, len(row))
		col = append(col, row...)

		s.formatRows = append(s.formatRows, col)
	}

	header := make([]string, len(s.header))
	for i := range s.header {
		header[i] = s.header[i]
	}

	s.t = baseTableTemplate(header)
	s.updateTableRows()
	return nil
}

// View draws the table and its menus and returns it as a string.
func (s *selectableTable) View() string {
	if s.done {
		return ""
	}

	tableStr := s.t.String()
	parts := strings.Split(tableStr, "\n")

	// These are the number of rows taken up by the table header and footer.
	headerLength := 3
	footerLength := 1

	for i, part := range parts {
		row := i + s.startIndex
		var selector string
		if i == 2 && s.startIndex > 0 {
			selector = lipgloss.NewStyle().SetString("↑").Bold(true).Foreground(lipgloss.Color(DarkGrey)).String()
		} else if i == len(parts)-footerLength && s.startIndex < len(s.formatRows)-s.size {
			selector = lipgloss.NewStyle().SetString("↓").Bold(true).Foreground(lipgloss.Color(DarkGrey)).String()
		} else if i == len(parts)-footerLength {
			selector = " "
		} else if i < headerLength {
			selector = " "
		} else if s.activeRows[s.filterMap[row-headerLength]] {
			selector = SuccessSymbol()
		} else if s.disabledRows[s.filterMap[row-headerLength]] {
			selector = ErrorSymbol()
		} else if s.currentRow+headerLength == row {
			selector = SetColor(Orange, ">", false)
		} else {
			selector = " "
		}

		parts[i] = fmt.Sprintf(" %s %s", selector, part)
	}

	filter := ColorPrintf(Format{Arg: "\n Filter |  %s\n", Color: Grey}, Format{Arg: s.filter, Color: Orange})

	helpEnter := Format{Color: Grey, Arg: "enter", Bold: true}
	helpSpace := Format{Color: Grey, Arg: "space", Bold: true}
	helpRight := Format{Color: Grey, Arg: "→", Bold: true}
	helpType := Format{Color: Grey, Arg: "type", Bold: true}
	helpLeft := Format{Color: Grey, Arg: "←", Bold: true}
	helpUp := Format{Color: Grey, Arg: "↑", Bold: true}
	helpDown := Format{Color: Grey, Arg: "↓", Bold: true}

	helpTmpl := Format{Color: DarkGrey, Arg: " %s to select; %s to confirm; %s to filter results.\n %s/%s to move; %s to select all; %s to select none."}
	help := ColorPrintf(helpTmpl, helpSpace, helpEnter, helpType, helpUp, helpDown, helpRight, helpLeft)

	return filter + strings.Join(parts, "\n") + "\n" + help
}

func (s *selectableTable) filterRows(updatePos bool) {
	s.formatRows = [][]string{}
	s.filterMap = map[int]int{}
	if updatePos {
		s.currentRow = 0
		s.startIndex = 0
	}
	index := 0
	for i, row := range s.rawRows {
		match := len(s.filter) == 0
		if !match {
			for _, col := range row {
				if strings.Contains(col, s.filter) {
					match = true
					break
				}
			}
		}

		if match {
			col := make([]string, 0, len(row))
			col = append(col, row...)
			s.formatRows = append(s.formatRows, col)

			s.filterMap[index] = i
			index++
		}
	}

	s.updateTableRows()
}

func (s *selectableTable) updateTableRows() {
	filter := table.NewFilter(table.NewStringData(s.formatRows...))
	filter = filter.Filter(func(row int) bool {
		match := row >= s.startIndex && row < s.startIndex+s.size
		s.rowStyle(row)

		return match
	})

	s.t = s.t.Data(filter)
}

func (s *selectableTable) handleRemoveEvent(remove RemoveMsg) (tea.Model, tea.Cmd) {
	newRows := make([][]string, 0, len(s.rawRows)-1)
	for i, row := range s.rawRows {
		if i == int(remove) {
			delete(s.activeRows, i)
			delete(s.disabledRows, i)

			continue
		}

		col := make([]string, 0, len(row))
		col = append(col, row...)
		newRows = append(newRows, col)
	}

	s.rawRows = newRows
	s.filterRows(true)

	return s, nil
}

func (s *selectableTable) handleInsertEvent(insert InsertMsg) (tea.Model, tea.Cmd) {
	s.rawRows = append(s.rawRows, insert)
	s.filterRows(false)

	return s, nil
}

func (s *selectableTable) handleDisableEvent(disable DisableMsg) (tea.Model, tea.Cmd) {
	// indexes to disable should always be absolute indexes and do not need to go through s.filterMap.
	delete(s.activeRows, int(disable))
	s.disabledRows[int(disable)] = true

	return s, nil
}

func (s *selectableTable) handleEnableEvent(enable EnableMsg) (tea.Model, tea.Cmd) {
	// indexes to enable should always be absolute indexes and do not need to go through s.filterMap.
	delete(s.disabledRows, int(enable))

	return s, nil
}

func (s *selectableTable) handleKeyEvent(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEsc:
		fallthrough
	case tea.KeyCtrlC:
		s.activeRows = map[int]bool{}
		fallthrough
	case tea.KeyEnter:
		s.done = true
		return s, tea.Quit

	case tea.KeyUp:
		if s.currentRow > 0 {
			s.currentRow--

			if s.currentRow < s.startIndex {
				s.startIndex--
			}
		}
	case tea.KeyDown:
		if s.currentRow < len(s.formatRows)-1 {
			s.currentRow++

			if s.currentRow > s.startIndex+(s.size-1) {
				s.startIndex++
			}
		}
	case tea.KeyLeft:
		s.activeRows = map[int]bool{}
	case tea.KeyRight:
		for i := range s.formatRows {
			if !s.disabledRows[s.filterMap[i]] {
				s.activeRows[s.filterMap[i]] = true
			}
		}

	case tea.KeySpace:
		if !s.disabledRows[s.filterMap[s.currentRow]] && len(s.formatRows) > 0 {
			if s.activeRows[s.filterMap[s.currentRow]] {
				delete(s.activeRows, s.filterMap[s.currentRow])
			} else {
				s.activeRows[s.filterMap[s.currentRow]] = true
			}
		}

	case tea.KeyBackspace:
		if len(s.filter) > 0 {
			s.filter = s.filter[:len(s.filter)-1]
		}
		s.filterRows(true)

	case tea.KeyDelete:
		if len(s.filter) > 0 {
			s.filter = s.filter[1:len(s.filter)]
		}
		s.filterRows(true)
	case tea.KeyRunes:
		for _, rune := range key.Runes {
			s.filter += string(rune)
		}

		s.filterRows(true)
	default:
		s.activeRows = map[int]bool{}
		s.done = true
		return s, tea.Quit
	}

	return s, nil
}

func (s *selectableTable) rowStyle(row int) {
	for col := range s.formatRows[row] {
		rawRowIndex := s.filterMap[row]
		textStyle := lipgloss.NewStyle().SetString(s.rawRows[rawRowIndex][col])
		if row == s.currentRow {
			if s.activeRows[s.filterMap[row]] {
				textStyle = textStyle.Bold(true).Foreground(lipgloss.Color(Green))
			} else if s.disabledRows[s.filterMap[row]] {
				textStyle = textStyle.Bold(true).Foreground(lipgloss.Color(Red))
			} else {
				textStyle = textStyle.Bold(true).Foreground(lipgloss.Color(White))
			}
		} else {
			if s.activeRows[s.filterMap[row]] {
				textStyle = textStyle.Bold(false).Foreground(lipgloss.Color(Green))
			} else if s.disabledRows[s.filterMap[row]] {
				textStyle = textStyle.Bold(false).Foreground(lipgloss.Color(Red))
			} else {
				textStyle = textStyle.Bold(false).Foreground(lipgloss.Color(Grey))
			}
		}

		s.formatRows[row][col] = textStyle.String()
	}
}
