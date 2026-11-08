package changelog

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"unicode"
)

const underlineMinLength = 3

// BlockKind identifies the structure of a changelog block.
type BlockKind uint8

const (
	BlockText BlockKind = iota
	BlockHeading
	BlockListItem
)

// Block is one structured line from a changelog document.
type Block struct {
	Kind  BlockKind
	Text  string
	Level int
}

// BlockScanner reads structured changelog blocks without limiting line length.
type BlockScanner struct {
	reader   *bufio.Reader
	pending  string
	block    Block
	err      error
	finished bool
}

// NewBlockScanner returns a scanner for headings, list items, and text.
func NewBlockScanner(reader io.Reader) *BlockScanner {
	return &BlockScanner{reader: bufio.NewReader(reader)}
}

// Scan advances to the next non-empty block.
func (scanner *BlockScanner) Scan() bool {
	for {
		line, ok := scanner.readLine()
		if !ok {
			if scanner.pending == "" {
				return false
			}
			scanner.block = classifyBlock(scanner.pending)
			scanner.pending = ""
			return true
		}
		if line == "" {
			if scanner.pending == "" {
				continue
			}
			scanner.block = classifyBlock(scanner.pending)
			scanner.pending = ""
			return true
		}
		if level, underline := underlineHeading(line); underline && headingCandidate(scanner.pending) {
			scanner.block = Block{Kind: BlockHeading, Text: scanner.pending, Level: level}
			scanner.pending = ""
			return true
		}
		if scanner.pending == "" {
			scanner.pending = line
			continue
		}
		scanner.block = classifyBlock(scanner.pending)
		scanner.pending = line
		return true
	}
}

// Block returns the most recent block produced by Scan.
func (scanner *BlockScanner) Block() Block {
	return scanner.block
}

// Err returns the first non-EOF read error.
func (scanner *BlockScanner) Err() error {
	return scanner.err
}

func (scanner *BlockScanner) readLine() (string, bool) {
	if scanner.finished {
		return "", false
	}
	text, err := scanner.reader.ReadString('\n')
	if err != nil {
		scanner.finished = true
		if !errors.Is(err, io.EOF) {
			scanner.err = err
		}
	}
	if text == "" && err != nil {
		return "", false
	}
	return strings.TrimSpace(text), true
}

func classifyBlock(line string) Block {
	if level, text, heading := prefixedHeading(line); heading {
		return Block{Kind: BlockHeading, Text: text, Level: level}
	}
	if listItem(line) {
		_, text, _ := strings.Cut(line, " ")
		return Block{Kind: BlockListItem, Text: strings.TrimSpace(text)}
	}
	return Block{Kind: BlockText, Text: line}
}

func prefixedHeading(line string) (int, string, bool) {
	marker := line[0]
	if marker != '#' && marker != '=' {
		return 0, "", false
	}
	level := 0
	for level < len(line) && line[level] == marker {
		level++
	}
	if level == len(line) || !unicode.IsSpace(rune(line[level])) {
		return 0, "", false
	}
	return level, strings.TrimSpace(line[level:]), true
}

func underlineHeading(line string) (int, bool) {
	if len(line) < underlineMinLength {
		return 0, false
	}
	level := strings.IndexByte("=-~^", line[0]) + 1
	if level == 0 {
		return 0, false
	}
	for index := 1; index < len(line); index++ {
		if line[index] != line[0] {
			return 0, false
		}
	}
	return level, true
}

func headingCandidate(line string) bool {
	return line != "" && !listItem(line) && !strings.HasPrefix(line, ".. ")
}

func listItem(line string) bool {
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return true
	}
	marker, _, found := strings.Cut(line, " ")
	if !found || (!strings.HasSuffix(marker, ".") && !strings.HasSuffix(marker, ")")) {
		return false
	}
	marker = strings.TrimSuffix(strings.TrimSuffix(marker, "."), ")")
	return marker != "" && strings.IndexFunc(marker, func(char rune) bool { return !unicode.IsDigit(char) }) == -1
}
