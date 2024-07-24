package dsearch

import (
	"fmt"

	fzf "github.com/junegunn/fzf/src"
)

///////////////////////////////////////////////////////////////////////////////

type FzfDelegate struct {
	query  chan string
	input  chan string
	output chan string
}

///////////////////////////////////////////////////////////////////////////////

func NewFzfDelegate() FzfDelegate {
	return FzfDelegate{
		query: make(chan string),
	}
}

///////////////////////////////////////////////////////////////////////////////

func (p *FzfDelegate) execute(query string, foundCb func(string), entryLoop func(chan string)) {
	p.input = make(chan string)
	p.output = make(chan string)
	go func() {
		for name := range p.output {
			foundCb(name)
		}
	}()
	go func() {
		entryLoop(p.input)
	}()
	filter(query, p.input, p.output)
}

///////////////////////////////////////////////////////////////////////////////

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
