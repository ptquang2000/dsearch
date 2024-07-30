package dsearch

import (
	"fmt"
	"sync"

	fzf "github.com/junegunn/fzf/src"
)

///////////////////////////////////////////////////////////////////////////////

type FzfStream chan string
type found func(string)
type read func(FzfStream)

type IFzfDelegate interface {
	ExecuteSync(string, found, read)
	ExecuteAsync(string, found, read) *sync.WaitGroup
}

type FzfConfig struct {
	exact      bool
	ignoreCase bool
	algo       int
}

type FzfDelegate struct {
	cfg FzfConfig
}

///////////////////////////////////////////////////////////////////////////////

func NewFzfDelegate(cfg FzfConfig) IFzfDelegate {
	return &FzfDelegate{cfg: cfg}
}

///////////////////////////////////////////////////////////////////////////////

func (p *FzfDelegate) ExecuteSync(q string, f found, r read) {
	input := make(FzfStream)
	output := make(FzfStream)
	var fin sync.WaitGroup
	fin.Add(1)
	go func() {
		for name := range output {
			f(name)
		}
		fin.Done()
	}()
	go func() {
		r(input)
		close(input)
	}()
	p.filter(q, input, output)
	close(output)
	fin.Wait()
}

///////////////////////////////////////////////////////////////////////////////

func (p *FzfDelegate) ExecuteAsync(q string, f found, r read) *sync.WaitGroup {

	input := make(FzfStream)
	output := make(FzfStream)
	var fin sync.WaitGroup
	fin.Add(1)
	go func() {
		for name := range output {
			f(name)
		}
		fin.Done()
	}()
	go func() {
		r(input)
		close(input)
	}()
	go func() {
		p.filter(q, input, output)
		close(output)
	}()
	return &fin
}

///////////////////////////////////////////////////////////////////////////////

func (p *FzfDelegate) filter(q string, i FzfStream, o FzfStream) int {
	args := []string{
		"--filter",
		fmt.Sprintf(`%s`, q),
		"--no-sort",
	}
	if p.cfg.exact {
		args = append(args, "--exact")
	}
	if p.cfg.ignoreCase {
		args = append(args, "--ignore-case")
	}
	if p.cfg.algo%2 == 1 {
		args = append(args, "--algo")
		args = append(args, "v1")
	}

	opts, err := fzf.ParseOptions(true, args)
	opts.Input = i
	opts.Output = o
	if err != nil {
		return -1
	}

	code, _ := fzf.Run(opts)
	return code
}
