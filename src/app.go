package dsearch

import (
	"code.rocketnine.space/tslocum/desktop"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/mnogu/go-calculator"

	"github.com/charlievieth/fastwalk"

	fzf "github.com/junegunn/fzf/src"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

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

	p := tea.NewProgram(newModel())
	if _, err := p.Run(); err != nil {
		log.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

type errMsg error
type EntryList []*desktop.Entry
type Applications map[string]string
type Files map[string]string

type DataReadyMsg struct{ choices []string }
type DelayedQueryMsg struct{}

type model struct {
	delegate *entriesDelegate

	isDelayed bool
	textInput textinput.Model
	err       error

	choices []string
	top     int
	cursor  int
}

func newModel() *model {
	ti := textinput.New()
	ti.Placeholder = "Searching ..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20
	ti.Prompt = "   "

	return &model{
		delegate:  newEntriesDelegate(),
		textInput: ti,
		top:       6,
		cursor:    0,
		err:       nil,
	}
}

func (m *model) Init() tea.Cmd {
	log.Println("BubbleTea init")
	return tea.Batch(
		tea.SetWindowTitle("DSearch"),
		textinput.Blink,
		m.delegate.outputStream(),
		m.delegate.inputStream("", false),
	)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Printf(`Received new key msg "%s"`, msg.String())
		if cmd := m.handleKey(msg.Type); cmd != nil {
			return m, cmd
		}
		if cmd := m.handleTextInput(msg); cmd != nil {
			return m, cmd
		}
		return m, nil
	case DataReadyMsg:
		log.Println("Data is ready")
		m.choices = msg.choices

		lastIndex := len(m.choices) - 1
		if m.cursor > lastIndex {
			m.cursor = lastIndex
		}
		if m.isDelayed {
			log.Println("There was a delayed query")
			m.isDelayed = false
			query := m.textInput.Value()
			return m, m.delegate.inputStream(query, m.isDelayed)
		}
		return m, m.delegate.inputStream("", true)
	case DelayedQueryMsg:
		log.Println("Query will be delayed until data is ready")
		m.isDelayed = true
		return m, nil
	case errMsg:
		m.err = msg
		return m, nil
	}
	return m, nil
}

func (m *model) handleTextInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	lastQuery := m.textInput.Value()
	m.textInput, cmd = m.textInput.Update(msg)
	if query := m.textInput.Value(); lastQuery != query {
		return tea.Batch(cmd, m.refreshEntries(query))
	}
	return cmd
}

func (m *model) refreshEntries(query string) tea.Cmd {
	return func() tea.Msg {
		select {
		case m.delegate.query <- query:
			return nil
		default:
			return DelayedQueryMsg{}
		}
	}
}

func (m *model) handleKey(key tea.KeyType) tea.Cmd {
	switch key {
	case tea.KeyCtrlC, tea.KeyEsc:
		return tea.Quit
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown:
		if m.cursor < len(m.choices)-1 {
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

func (m *model) View() string {
	sb := new(strings.Builder)
	sb.WriteString(fmt.Sprintf("%s\n\n", m.textInput.View()))

	start := 0
	end := m.top
	if m.cursor >= m.top {
		start = m.cursor + 1 - m.top
		end = m.cursor + 1
	}
	for i := start; i < end && i < len(m.choices); i++ {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		sb.WriteString(fmt.Sprintf("%s %s\n", cursor, m.choices[i]))
	}

	sb.WriteString("\nPress Esc to quit.\n")

	return sb.String()
}

type entries struct {
	loaded bool
	apps   Applications
	files  Files
}
type entriesDelegate struct {
	data           entries
	entries        []string
	trackedEntries []string

	mutex     sync.Mutex
	refreshed chan int
	query     chan string
	fzfIn     chan string
	fzfOut    chan string
}

func newEntriesDelegate() *entriesDelegate {
	return &entriesDelegate{
		data:      entries{},
		refreshed: make(chan int),
		query:     make(chan string),
		fzfOut:    make(chan string),
	}
}

func (p *entriesDelegate) outputStream() tea.Cmd {
	return p.handleOutStream
}

func (p *entriesDelegate) handleOutStream() tea.Msg {
	isRefreshed := false
	for {
		select {
		case entry := <-p.fzfOut:
			p.mutex.Lock()
			if isRefreshed {
				isRefreshed = false
				p.entries = nil
			}
			p.entries = append(p.entries, entry)
			p.mutex.Unlock()
		case <-p.refreshed:
			isRefreshed = true
		default:
			runtime.Gosched()
		}
	}
}

func (p *entriesDelegate) inputStream(query string, wait bool) tea.Cmd {
	return func() tea.Msg {
		if wait {
			log.Println("Waiting for next query")
			query = <-p.query
		}
		log.Printf(`Received new query: "%s"`, query)
		p.fzfIn = make(chan string)
		go func() {
			loadCalculator(query, p.fzfIn)
			loadApplications(&p.data, p.fzfIn)
			loadFiles(&p.data, p.fzfIn)

			p.data.loaded = true
			log.Println("Finished loading data")
			close(p.fzfIn)
		}()
		p.refreshed <- filter(query, p.fzfIn, p.fzfOut)
		slices.Sort(p.entries)

		log.Println("Sent refresh signal")
		return DataReadyMsg{choices: p.entries}
	}
}

func loadCalculator(expr string, fzfIn chan string) {
	cal, err := calculator.Calculate(expr)
	if err != nil {
		return
	}

	if cal == math.Trunc(cal) {
		fzfIn <- fmt.Sprintf(`%s = %d`, expr, int64(cal))
	} else {
		fzfIn <- fmt.Sprintf(`%s = %f`, expr, cal)
	}
}

func loadApplications(data *entries, fzfIn chan string) {
	if data.loaded {
		for app := range data.apps {
			fzfIn <- app
		}
		return
	}

	dirs, err := desktop.Scan(desktop.DataDirs())
	if err != nil {
		log.Println("Failed to scan applications")
		return
	}

	for _, entries := range dirs {
		if len(entries) == 0 {
			continue
		}
		l := EntryList(entries)
		data.apps = traverseEntryList(l, fzfIn)
	}
}

func loadFiles(data *entries, fzfIn chan string) bool {
	if data.loaded {
		for file := range data.files {
			fzfIn <- file
		}
		return true
	}

	root, err := os.UserHomeDir()
	if err != nil {
		log.Println("Failed to locate home directory")
		return false
	}

	var mutex sync.Mutex
	data.files = make(Files)
	fn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("Error during walking %s: %v\n", path, err)
			return nil // returning the error stops iteration
		}

		relativePath := strings.Replace(path, root, "", 1)
		if !d.IsDir() && !isHiddenFile(relativePath) {
			mutex.Lock()
			fzfIn <- path
			data.files[path] = path
			mutex.Unlock()
		} else if d.IsDir() && isHiddenDir(relativePath) {
			return fastwalk.SkipDir
		}
		return err
	}

	walkCfg := fastwalk.Config{
		Follow:  true,
		ToSlash: fastwalk.DefaultToSlash(),
	}
	return fastwalk.Walk(&walkCfg, root, fn) == nil
}

func isHiddenFile(path string) bool {
	fileName := filepath.Base(path)
	if len(fileName) > 1 && fileName[0] == '.' {
		return true
	}
	return false
}

func isHiddenDir(path string) bool {
	tokens := strings.Split(path, "/")
	for _, token := range tokens {
		if len(token) > 1 && token[0] == '.' {
			return true
		}
	}
	return false
}

func traverseEntryList(entries EntryList, fzfIn chan string) Applications {
	apps := make(Applications)
	for _, entry := range entries {
		isApp := strings.Compare(entry.Type.String(), "Application") == 0
		if !isApp || entry.Terminal {
			continue
		}
		fzfIn <- entry.Name
		apps[entry.Name] = entry.Exec
	}
	return apps
}

func filter(query string, input chan string, output chan string) int {
	args := []string{
		"--filter",
		fmt.Sprintf(`%s`, query),
		"--no-sort",
		"--exact",
		"--ignore-case",
	}
	opts, err := fzf.ParseOptions(true, args)
	opts.Input = input
	opts.Output = output
	if err != nil {
		return -1
	}
	code, _ := fzf.Run(opts)
	return code
}
