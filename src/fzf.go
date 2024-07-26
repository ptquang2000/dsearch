package dsearch

import (
	"fmt"

	fzf "github.com/junegunn/fzf/src"
)

///////////////////////////////////////////////////////////////////////////////

type FzfStream chan string
type FzfDelegate struct {
	input  FzfStream
	output FzfStream
}

///////////////////////////////////////////////////////////////////////////////

func NewFzfDelegate() FzfDelegate {
	return FzfDelegate{}
}

///////////////////////////////////////////////////////////////////////////////

func (p *FzfDelegate) execute(
	query string,
	foundCb func(string),
	entryLoop func(FzfStream)) {
	p.input = make(FzfStream)
	p.output = make(FzfStream)
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
