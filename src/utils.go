package dsearch

import (
	"bufio"
	"fmt"
	"io/fs"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"code.rocketnine.space/tslocum/desktop"

	"github.com/mnogu/go-calculator"

	"github.com/charlievieth/fastwalk"
)

////////////////////////////////////////////////////////////////////////////////

type EntryList []*desktop.Entry

///////////////////////////////////////////////////////////////////////////////

func loadCalculator(expr string) *Entry {
	cal, err := calculator.Calculate(expr)
	if err != nil {
		return nil
	}

	entry := Entry{execute: func() {}}
	if cal == math.Trunc(cal) {
		entry.name = fmt.Sprintf(`%s = %d`, expr, int64(cal))
	} else {
		entry.name = fmt.Sprintf(`%s = %f`, expr, cal)
	}
	return &entry
}

///////////////////////////////////////////////////////////////////////////////

func loadApplications(entryChan chan *Entry) {
	for _, dir := range desktop.DataDirs() {
		walkDataDir(dir, entryChan)
	}
}

///////////////////////////////////////////////////////////////////////////////

func walkDataDir(root string, entryChan chan *Entry) {
	fn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf(`Error during walking %s: %v\n`, path, err)
			return nil // returning the error stops iteration
		}

		if !d.IsDir() {
			parseDesktopFile(path, entryChan)
		}
		return err
	}

	fastwalk.Walk(
		&fastwalk.Config{
			Follow:  true,
			ToSlash: fastwalk.DefaultToSlash(),
		},
		root,
		fastwalk.IgnorePermissionErrors(fn))
}

///////////////////////////////////////////////////////////////////////////////

func parseDesktopFile(path string, entryChan chan *Entry) {
	parts := strings.Split(path, ".")
	if len(parts) < 2 || parts[len(parts)-1] != "desktop" {
		return
	}

	f, err := os.Open(path)
	if err != nil {
		log.Printf(`Failed to open file %s, err:%v`, path, err)
	}
	buf := make([]byte, 0, 64*1024)
	reader := bufio.NewReader(f)
	entry, err := desktop.Parse(reader, buf)
	if err != nil {
		log.Printf(`Failed to parse file %s, err %v`, path, err)
		return
	}
	if entry != nil && entry.Type == desktop.Application {
		entryChan <- buildAppEntry(path, entry)
	}
}

///////////////////////////////////////////////////////////////////////////////

func buildAppEntry(path string, entry *desktop.Entry) *Entry {
	action := func() {
		cmd := exec.Command("gio", "launch", path)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Foreground: false,
			Setsid:     true,
		}
		if err := cmd.Start(); err != nil {
			log.Fatalf(
				`Failed to exec %s, err:%v`,
				cmd.String(),
				err)
		}
		cmd.Process.Release()
	}
	return &Entry{name: entry.Name, execute: action}
}

///////////////////////////////////////////////////////////////////////////////

func loadFiles(entryChan chan *Entry, hidden bool) bool {
	root, err := os.UserHomeDir()
	if err != nil {
		log.Printf(`Failed to locate home directory`)
		return false
	}

	fn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf(`Error during walking %s: %v\n`, path, err)
			return nil // returning the error stops iteration
		}

		relativePath := strings.Replace(path, root, "", 1)
		if !d.IsDir() && (hidden || !isHiddenFile(relativePath)) {
			entryChan <- buildFileEntry(path)
		} else if d.IsDir() && !hidden && isHiddenDir(relativePath) {
			return fastwalk.SkipDir
		}
		return err
	}

	walkCfg := fastwalk.Config{
		Follow:  false,
		ToSlash: fastwalk.DefaultToSlash(),
	}
	return fastwalk.Walk(
		&walkCfg,
		root,
		fastwalk.IgnorePermissionErrors(fn)) == nil
}

///////////////////////////////////////////////////////////////////////////////

func buildFileEntry(path string) *Entry {
	action := func() {
		cmd := exec.Command("xdg-open", path)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Foreground: false,
			Setsid:     true,
		}
		if err := cmd.Start(); err != nil {
			log.Fatalf(
				`Failed to exec %s, err:%v`,
				cmd.String(),
				err)
		}
		cmd.Process.Release()
	}
	return &Entry{name: path, execute: action}
}

///////////////////////////////////////////////////////////////////////////////

func isHiddenFile(path string) bool {
	fileName := filepath.Base(path)
	if len(fileName) > 1 && fileName[0] == '.' {
		return true
	}
	return false
}

///////////////////////////////////////////////////////////////////////////////

func isHiddenDir(path string) bool {
	tokens := strings.Split(path, "/")
	for _, token := range tokens {
		if len(token) > 1 && token[0] == '.' {
			return true
		}
	}
	return false
}

///////////////////////////////////////////////////////////////////////////////
