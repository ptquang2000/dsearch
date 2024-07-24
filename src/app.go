package dsearch

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

///////////////////////////////////////////////////////////////////////////////

type model struct {
	ready  bool
	height int
	width  int

	entries   Entries
	textInput textinput.Model

	sigRefreshed chan RefreshedMsg
	head         *Entry
	count        int
	cursor       int
}
type RefreshedMsg struct {
	head  *Entry
	count int
}
type EntriesLoadedMsg struct {
	head  *Entry
	count int
}
type FilteredMsg struct {
	head  *Entry
	count int
}
type NewQueryMsg struct {
	query string
}

///////////////////////////////////////////////////////////////////////////////

func Run() {
	if len(os.Getenv("DEBUG")) > 0 {
		f, err := tea.LogToFile("dsearch.log", "dsearch")
		if err != nil {
			log.Println("fatal:", err)
			os.Exit(1)
		}
		defer f.Close()
	} else {
		log.SetOutput(io.Discard)
	}

	p := tea.NewProgram(&model{
		entries:      NewEntries(),
		sigRefreshed: make(chan RefreshedMsg),
		cursor:       0,
	})
	if _, err := p.Run(); err != nil {
		log.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

///////////////////////////////////////////////////////////////////////////////

func (m *model) Init() tea.Cmd {
	log.Println("BubbleTea init")
	return tea.Batch(
		tea.SetWindowTitle("DSearch"),
		textinput.Blink,
		onRefreshView(m.sigRefreshed),
		m.entries.LoadEntries(m.sigRefreshed),
	)
}

///////////////////////////////////////////////////////////////////////////////

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if cmd := m.onKeyChanged(msg.Type); cmd != nil {
			return m, cmd
		}
		if cmd := m.onTextInputChanged(msg); cmd != nil {
			return m, cmd
		}
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		if !m.ready {
			m.onWindowReady()
			m.ready = true
		}
	case RefreshedMsg:
		m.head = msg.head
		m.count = msg.count
		return m, onRefreshView(m.sigRefreshed)
	case EntriesLoadedMsg:
		log.Printf("Finished to load all entries")
		m.head = msg.head
		m.count = msg.count
		return m, onRefreshView(m.sigRefreshed)
	case NewQueryMsg:
		return m, m.entries.FilterEntry(m.sigRefreshed, msg.query)
	default:
		log.Println("Update")
	}
	return m, nil
}

///////////////////////////////////////////////////////////////////////////////

func onRefreshView(sigRefreshed chan RefreshedMsg) tea.Cmd {
	return func() tea.Msg {
		return RefreshedMsg(<-sigRefreshed)
	}
}

///////////////////////////////////////////////////////////////////////////////

func (m *model) onWindowReady() {
	ti := textinput.New()
	ti.Placeholder = "Searching ..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = m.width
	ti.Prompt = "   "
	m.textInput = ti
}

///////////////////////////////////////////////////////////////////////////////

func (m *model) onTextInputChanged(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	lastQuery := m.textInput.Value()
	m.textInput, cmd = m.textInput.Update(msg)
	if query := m.textInput.Value(); lastQuery != query {
		return tea.Batch(cmd, m.onFilterRequested(query))
	}
	return nil
}

func (m *model) onFilterRequested(query string) tea.Cmd {
	return func() tea.Msg {
		m.entries.Stop()
		return NewQueryMsg{query: query}
	}
}

///////////////////////////////////////////////////////////////////////////////

func (m *model) onKeyChanged(key tea.KeyType) tea.Cmd {
	switch key {
	case tea.KeyCtrlC, tea.KeyEsc:
		return tea.Quit
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown:
		if m.cursor < m.count-1 {
			m.cursor++
		}
	case tea.KeyEnter:
		//_, ok := m.selected[m.cursor]
		//if ok {
		//	delete(m.selected, m.cursor)
		//} else {
		//	m.selected[m.cursor] = struct{}{}
		//}
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////

func (m *model) View() string {
	if !m.ready {
		return "\n Inializing ..."
	}

	sb := new(strings.Builder)

	sb.WriteString(fmt.Sprintf("\n\n%s\n\n", m.textInput.View()))

	limit := m.height - 6
	start := max(0, m.cursor+1-limit)
	end := max(limit, m.cursor+1)
	iter := m.head
	for i := 0; i < start; i++ {
		if iter == nil {
			break
		}
		iter = iter.next
	}
	for i := start; i < m.count && i < end; i++ {
		if iter == nil {
			break
		}
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		sb.WriteString(fmt.Sprintf("%s %s\n", cursor, iter.name))
		iter = iter.next
	}

	sb.WriteString("\nPress Esc to quit.\n")

	return sb.String()
}

///////////////////////////////////////////////////////////////////////////////
