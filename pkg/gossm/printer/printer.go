package printer

import (
	"fmt"
	"github.com/glassechidna/gossm/pkg/gossm"
	"github.com/logrusorgru/aurora"
	"github.com/mitchellh/go-wordwrap"
	"github.com/nsf/termbox-go"
	"io"
	"os"
	"strings"
)

type Printer struct {
	Out       io.Writer
	outColors []aurora.Color
	Err       io.Writer
	errColors []aurora.Color
	Quiet     bool
}

func New() *Printer {
	return &Printer{
		Out:       os.Stdout,
		outColors: []aurora.Color{aurora.GreenFg},
		Err:       os.Stderr,
		errColors: []aurora.Color{aurora.RedFg},
		Quiet:     false,
	}
}

func (p *Printer) PrintInfo(command string, resp *gossm.DoitResponse) {
	p.printInfo("Command: ", command)
	p.printInfo("Command ID: ", resp.CommandId)

	instanceIds := resp.InstanceIds.InstanceIds
	prefix := fmt.Sprintf("Running command on %d instances: ", len(instanceIds))
	p.printInfo(prefix, fmt.Sprintf("%+v", instanceIds))
}

func (p *Printer) printInfo(prefix, info string) {
	if !p.Quiet {
		faint := aurora.GrayFg
		blue := aurora.BlueFg
		prefixstr := aurora.Colorize(prefix, blue).String()
		infostr := aurora.Colorize(info, faint).String()
		_, _ = fmt.Fprint(os.Stderr, prefixstr)
		_, _ = fmt.Fprintln(os.Stderr, infostr)
	}
}

func (p *Printer) Print(msg gossm.SsmMessage) {
	if len(msg.StdoutChunk) > 0 {
		p.print(p.Out, p.outColors[0], msg, msg.StdoutChunk)
	}

	if len(msg.StderrChunk) > 0 {
		p.print(p.Err, p.errColors[0], msg, msg.StderrChunk)
	}

	if !p.Quiet {
		_, _ = fmt.Fprintln(p.Out) // split em out
	}

	if msg.Error != nil {
		panic(msg.Error)
	}
}

func (p *Printer) print(w io.Writer, prefixColor aurora.Color, msg gossm.SsmMessage, payload string) {
	if p.Quiet {
		_, _ = fmt.Fprintln(w, payload)
		return
	}

	windowWidth := 80

	if err := termbox.Init(); err == nil {
		windowWidth, _ = termbox.Size()
		termbox.Close()
	}

	prefix := aurora.Colorize(fmt.Sprintf("[%s] ", msg.InstanceId), prefixColor).String()

	outputWidth := windowWidth - len(prefix)
	wrapped := wordwrap.WrapString(payload, uint(outputWidth))
	lines := strings.Split(wrapped, "\n")

	for idx, line := range lines {
		if !(len(line) == 0 && idx == len(lines)-1) {
			_, _ = fmt.Fprintf(w, "%s%s\n", prefix, line)
		}
	}
}
