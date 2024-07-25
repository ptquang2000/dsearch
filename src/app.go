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

type SigRefresh chan RefreshingMsg
type model struct {
	ready  bool
	height int
	width  int

	entries   Entries
	textInput textinput.Model

	sigRefresh SigRefresh
	head       *Entry
	count      int32
	cursor     int32
}
type RefreshingMsg struct {
	head  *Entry
	count int32
}
type LoadedMsg struct{}
type FilteredMsg struct {
	head  *Entry
	count int32
}
type StoppedMsg struct{}
type QueryMsg struct {
	query string
}
type SelectedMsg struct {
	entry *Entry
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
		entries:    NewEntries(),
		sigRefresh: make(SigRefresh),
		cursor:     0,
	})
	if _, err := p.Run(); err != nil {
		log.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

///////////////////////////////////////////////////////////////////////////////

func (m *model) Init() tea.Cmd {
	log.Println("Initializing")
	return tea.Batch(
		tea.SetWindowTitle("DSearch"),
		textinput.Blink,
		onViewRefreshed(m.sigRefresh),
		m.entries.LoadEntries(m.sigRefresh),
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
	case RefreshingMsg:
		m.head = msg.head
		m.count = msg.count
		return m, onViewRefreshed(m.sigRefresh)
	case LoadedMsg:
		log.Println("Finished to load all entries")
		return m, onViewRefreshed(m.sigRefresh)
	case QueryMsg:
		log.Println("Received new query: ", msg.query)
		return m, m.entries.FilterEntry(m.sigRefresh, msg.query)
	case FilteredMsg:
		log.Println("Finished to filter query")
		m.head = msg.head
		m.count = msg.count
		return m, onViewRefreshed(m.sigRefresh)
	case StoppedMsg:
		log.Println("Filter execution was stopped")
	case SelectedMsg:
		log.Println("Select entry ", msg.entry.name)
		if !m.entries.SelectEntry(msg.entry) {
			log.Println("Failed to execute entry ", msg.entry.name)
		}
		return m, tea.Quit
	default:
		log.Println("Update")
	}
	return m, nil
}

///////////////////////////////////////////////////////////////////////////////

func onViewRefreshed(sigRefreshed chan RefreshingMsg) tea.Cmd {
	return func() tea.Msg {
		return RefreshingMsg(<-sigRefreshed)
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
		m.entries.StopFilter()
		return QueryMsg{query: query}
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
		head := m.head
		for i := int32(0); i < m.cursor && head != nil; i++ {
			head = head.fnext
		}
		return onSelectedEntry(head)
	}
	return nil
}

func onSelectedEntry(entry *Entry) tea.Cmd {
	return func() tea.Msg { return SelectedMsg{entry: entry} }
}

///////////////////////////////////////////////////////////////////////////////

func (m *model) View() string {
	if !m.ready {
		return "\n Inializing ..."
	}

	sb := new(strings.Builder)

	sb.WriteString(fmt.Sprintf("\n%s\n\n", m.textInput.View()))

	limit := int32(m.height) - 6
	start := max(int32(0), m.cursor+1-limit)
	end := max(limit, m.cursor+1)

	head := m.head
	for i := int32(0); i < start && head != nil; i++ {
		head = head.fnext
	}
	for i := start; i < m.count && i < end && head != nil; i++ {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		sb.WriteString(fmt.Sprintf("%s %s\n", cursor, head.name))
		head = head.fnext
	}

	sb.WriteString("\nPress Esc to quit.\n")

	return sb.String()
}

///////////////////////////////////////////////////////////////////////////////
