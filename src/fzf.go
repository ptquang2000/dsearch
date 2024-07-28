package dsearch

import (
	"fmt"
	"sync"

	fzf "github.com/junegunn/fzf/src"
)

///////////////////////////////////////////////////////////////////////////////

type FzfStream chan string
type FzfDelegate struct{}

///////////////////////////////////////////////////////////////////////////////

func (p *FzfDelegate) Execute(
	query string,
	foundCb func(string),
	entryLoop func(FzfStream)) {
	input := make(FzfStream)
	output := make(FzfStream)
	var fin sync.WaitGroup
	fin.Add(1)
	go func() {
		for name := range output {
			foundCb(name)
		}
		fin.Done()
	}()
	go func() {
		entryLoop(input)
		close(input)
	}()
	filter(query, input, output)
	close(output)
	fin.Wait()
}

///////////////////////////////////////////////////////////////////////////////

func filter(query string, input FzfStream, output FzfStream) int {
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
