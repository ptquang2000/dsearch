package dsearch

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

///////////////////////////////////////////////////////////////////////////////

type SigRefresh chan RefreshingMsg
type model struct {
	ready  bool
	height int
	width  int

	manager   IEntryManager
	textInput textinput.Model

	refreshCon SigRefresh
	entries    IEntryLinkedList
	cursor     int
}
type RefreshingMsg struct{ list IEntryLinkedList }
type LoadedMsg struct{}
type FilteredMsg struct{ list IEntryLinkedList }
type StoppedMsg struct{}
type QueryMsg struct {
	query string
}
type SelectedMsg struct {
	entry *EntryNode
}

///////////////////////////////////////////////////////////////////////////////

func isDebug() bool {
	debugFlag := flag.Bool("DEBUG", false, "Debug Mode")
	flag.Parse()
	return *debugFlag || len(os.Getenv("DEBUG")) > 0
}

///////////////////////////////////////////////////////////////////////////////

func Run() {
	if homeDir, err := os.UserHomeDir(); isDebug() && err == nil {
		dir := fmt.Sprintf(`%s/.dsearch.log`, homeDir)
		f, err := tea.LogToFile(dir, "dsearch")
		if err != nil {
			log.Printf(`Failed to log to file, err: %v`, err)
			os.Exit(1)
		}
		defer f.Close()
	} else {
		log.SetOutput(io.Discard)
	}

	refreshSignal := make(SigRefresh)
	p := tea.NewProgram(&model{
		manager:    NewEntryManager(refreshSignal),
		refreshCon: refreshSignal,
		cursor:     0,
	})
	if _, err := p.Run(); err != nil {
		log.Printf(`Alas, there's been an error: %v`, err)
		os.Exit(1)
	}
}

///////////////////////////////////////////////////////////////////////////////

func (m *model) Init() tea.Cmd {
	log.Printf(`Initializing`)
	return tea.Batch(
		tea.SetWindowTitle("DSearch"),
		textinput.Blink,
		onViewRefreshed(m.refreshCon),
		m.manager.LoadEntries(
			func(c chan *Entry) { loadApplications(c) },
			func(c chan *Entry) { loadFiles(c, true) }),
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
		m.entries = msg.list
		m.cursor = max(min(m.cursor, m.entries.len()-1), 0)
		return m, onViewRefreshed(m.refreshCon)
	case LoadedMsg:
		log.Printf(`Finished to load all entries`)
		return m, onViewRefreshed(m.refreshCon)
	case QueryMsg:
		log.Printf(`Received new query: %s`, msg.query)
		return m, m.manager.FilterEntry(msg.query)
	case FilteredMsg:
		log.Printf(`Finished to filter query`)
		m.entries = msg.list
		m.cursor = max(min(m.cursor, m.entries.len()-1), 0)
		return m, onViewRefreshed(m.refreshCon)
	case StoppedMsg:
		log.Printf(`Filter execution was stopped`)
	case SelectedMsg:
		name := msg.entry.value()
		log.Printf(`Select entry %s`, name)
		msg.entry.execute()
		return m, tea.Quit
	default:
		log.Printf(`Uknown update |%s|`, msg)
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
	ti.Prompt = " "
	ti.KeyMap.WordForward = key.NewBinding(key.WithKeys("ctrl+right"))
	ti.KeyMap.DeleteWordForward = key.NewBinding(key.WithKeys("\x1b[3;5~"))
	ti.KeyMap.WordBackward = key.NewBinding(key.WithKeys("ctrl+left"))
	ti.KeyMap.DeleteWordBackward = key.NewBinding(key.WithKeys("ctrl+h"))
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
		m.manager.StopFilter()
		return QueryMsg{query: query}
	}
}

///////////////////////////////////////////////////////////////////////////////

func (m *model) onKeyChanged(key tea.KeyType) tea.Cmd {
	switch key {
	case tea.KeyCtrlC, tea.KeyEsc:
		return tea.Quit
	case tea.KeyUp, tea.KeyCtrlP:
		m.cursor--
		m.cursor = max(m.cursor, 0)
	case tea.KeyDown, tea.KeyCtrlN:
		m.cursor++
		m.cursor = min(m.cursor, m.entries.len()-1)
	case tea.KeyEnter:
		head := m.entries.begin()
		for i := 0; i < m.cursor && head != nil; i++ {
			head = head.next
		}
		return onSelectedEntry(head)
	default:
		log.Printf(`Received keys: |%s|`, key)
	}
	return nil
}

func onSelectedEntry(entry *EntryNode) tea.Cmd {
	return func() tea.Msg { return SelectedMsg{entry: entry} }
}

///////////////////////////////////////////////////////////////////////////////

func (m *model) View() string {
	if !m.ready || m.entries == nil {
		return "\n Inializing ..."
	}

	sb := new(strings.Builder)

	sb.WriteString(fmt.Sprintf("\n %s\n", m.textInput.View()))

	limit := m.height - 6
	start := max(0, m.cursor+1-limit)
	end := max(limit, m.cursor+1)

	count := m.entries.len()
	iter := m.entries.begin()
	for i := 0; i < count && i < start && iter != nil; i++ {
		iter = iter.next
	}
	for i := start; i < count && i < end && iter != nil; i++ {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		sb.WriteString(fmt.Sprintf("\n %s %s", cursor, iter.value()))
		iter = iter.next
	}

	sb.WriteString("\n\n Press Esc to quit.\n")

	return sb.String()
}

///////////////////////////////////////////////////////////////////////////////
