package printer

import (
	"bytes"
	"github.com/glassechidna/gossm/pkg/gossm"
	"github.com/logrusorgru/aurora"
	"github.com/magiconair/properties/assert"
	"testing"
)

func TestPrintWrappedWithNewLineAtEOF(t *testing.T) {
	buf := &bytes.Buffer{}
	p := New()

	p.print(buf, aurora.BlueFg, gossm.SsmMessage{}, "hello world\n")
	str := buf.String()

	expected := "\x1b[34m[] \x1b[0mhello world\n"
	assert.Equal(t, str, expected)
}

func TestPrintWrappedWithoutNewLineAtEOF(t *testing.T) {
	buf := &bytes.Buffer{}
	p := New()

	p.print(buf, aurora.BlueFg, gossm.SsmMessage{}, "hello world")
	str := buf.String()

	expected := "\x1b[34m[] \x1b[0mhello world\n"
	assert.Equal(t, str, expected)
}
