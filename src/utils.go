package dsearch

import (
	"code.rocketnine.space/tslocum/desktop"
	"fmt"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/mnogu/go-calculator"

	"github.com/charlievieth/fastwalk"
)

////////////////////////////////////////////////////////////////////////////////

type EntryList []*desktop.Entry

///////////////////////////////////////////////////////////////////////////////

func loadCalculator(expr string, stream FzfStream) {
	cal, err := calculator.Calculate(expr)
	if err != nil {
		return
	}

	if cal == math.Trunc(cal) {
		stream <- fmt.Sprintf(`%s = %d`, expr, int64(cal))
	} else {
		stream <- fmt.Sprintf(`%s = %f`, expr, cal)
	}
}

///////////////////////////////////////////////////////////////////////////////

func loadApplications(entryChan chan *Entry) {
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
		traverseEntryList(l, entryChan)
	}
}

///////////////////////////////////////////////////////////////////////////////

func loadFiles(entryChan chan *Entry, hidden bool) bool {
	root, err := os.UserHomeDir()
	if err != nil {
		log.Println("Failed to locate home directory")
		return false
	}

	fn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("Error during walking %s: %v\n", path, err)
			return nil // returning the error stops iteration
		}

		relativePath := strings.Replace(path, root, "", 1)
		if !d.IsDir() && (hidden || !isHiddenFile(relativePath)) {
			entryChan <- &Entry{name: path, action: nil}
		} else if d.IsDir() && !hidden && isHiddenDir(relativePath) {
			return fastwalk.SkipDir
		}
		return err
	}

	walkCfg := fastwalk.Config{
		Follow:  true,
		ToSlash: fastwalk.DefaultToSlash(),
	}
	return fastwalk.Walk(
		&walkCfg,
		root,
		fastwalk.IgnorePermissionErrors(fn)) == nil
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

func traverseEntryList(entries EntryList, entryChan chan *Entry) {
	for _, entry := range entries {
		isApp := entry.Type.String() == "Application"
		if !isApp || entry.Terminal {
			continue
		}
		entryChan <- &Entry{name: entry.Name, action: nil}
	}
}
