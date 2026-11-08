package changelog

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
)

func TestBlockScanner(t *testing.T) {
	input := `# Release notes

== Security

- Reject unsafe paths
1. Rotate signing keys

Fixed
-----

Plain text

.. note:: directive
~~~
`
	want := []Block{
		{Kind: BlockHeading, Text: "Release notes", Level: 1},
		{Kind: BlockHeading, Text: "Security", Level: 2},
		{Kind: BlockListItem, Text: "Reject unsafe paths"},
		{Kind: BlockListItem, Text: "Rotate signing keys"},
		{Kind: BlockHeading, Text: "Fixed", Level: 2},
		{Kind: BlockText, Text: "Plain text"},
		{Kind: BlockText, Text: ".. note:: directive"},
		{Kind: BlockText, Text: "~~~"},
	}

	scanner := NewBlockScanner(strings.NewReader(input))
	var got []Block
	for scanner.Scan() {
		got = append(got, scanner.Block())
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("blocks = %#v, want %#v", got, want)
	}
}

func TestBlockScannerSupportsLongLines(t *testing.T) {
	longLine := strings.Repeat("x", 1<<20) + " security"
	scanner := NewBlockScanner(strings.NewReader(longLine))
	if !scanner.Scan() {
		t.Fatalf("Scan() = false, error = %v", scanner.Err())
	}
	block := scanner.Block()
	if block.Kind != BlockText || block.Text != longLine {
		t.Fatalf("block kind = %d, text length = %d", block.Kind, len(block.Text))
	}
	if scanner.Scan() {
		t.Fatal("unexpected second block")
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestBlockScannerReportsReadErrorsAfterPartialData(t *testing.T) {
	wantErr := errors.New("read failed")
	scanner := NewBlockScanner(&partialErrorReader{data: "security update", err: wantErr})
	if !scanner.Scan() {
		t.Fatalf("Scan() = false, error = %v", scanner.Err())
	}
	if block := scanner.Block(); block.Kind != BlockText || block.Text != "security update" {
		t.Fatalf("block = %#v", block)
	}
	if scanner.Scan() {
		t.Fatal("unexpected second block")
	}
	if !errors.Is(scanner.Err(), wantErr) {
		t.Fatalf("error = %v, want %v", scanner.Err(), wantErr)
	}
}

type partialErrorReader struct {
	data string
	err  error
}

func (reader *partialErrorReader) Read(buffer []byte) (int, error) {
	if reader.data == "" {
		return 0, reader.err
	}
	count := copy(buffer, reader.data)
	reader.data = reader.data[count:]
	return count, reader.err
}

var _ io.Reader = (*partialErrorReader)(nil)
