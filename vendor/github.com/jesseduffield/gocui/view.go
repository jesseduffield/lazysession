// Copyright 2014 The gocui Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gocui

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-errors/errors"

	"github.com/jesseduffield/termbox-go"
	"github.com/mattn/go-runewidth"
	"github.com/sirupsen/logrus"
)

// Constants for overlapping edges
const (
	TOP    = 1 // view is overlapping at top edge
	BOTTOM = 2 // view is overlapping at bottom edge
	LEFT   = 4 // view is overlapping at left edge
	RIGHT  = 8 // view is overlapping at right edge
)

// A View is a window. It maintains its own internal buffer and cursor
// position.
type View struct {
	name           string
	x0, y0, x1, y1 int
	ox, oy         int
	cx, cy         int
	lines          [][]cell
	readOffset     int
	readCache      string

	tainted   bool       // marks if the viewBuffer must be updated
	viewLines []viewLine // internal representation of the view's buffer

	ei *escapeInterpreter // used to decode ESC sequences on Write

	// BgColor and FgColor allow to configure the background and foreground
	// colors of the View.
	BgColor, FgColor Attribute

	// SelBgColor and SelFgColor are used to configure the background and
	// foreground colors of the selected line, when it is highlighted.
	SelBgColor, SelFgColor Attribute

	// If Editable is true, keystrokes will be added to the view's internal
	// buffer at the cursor position.
	Editable bool

	// Editor allows to define the editor that manages the edition mode,
	// including keybindings or cursor behaviour. DefaultEditor is used by
	// default.
	Editor Editor

	// Overwrite enables or disables the overwrite mode of the view.
	Overwrite bool

	// If Highlight is true, Sel{Bg,Fg}Colors will be used
	// for the line under the cursor position.
	Highlight bool

	// If Frame is true, a border will be drawn around the view.
	Frame bool

	// If Wrap is true, the content that is written to this View is
	// automatically wrapped when it is longer than its width. If true the
	// view's x-origin will be ignored.
	Wrap bool

	// If Autoscroll is true, the View will automatically scroll down when the
	// text overflows. If true the view's y-origin will be ignored.
	Autoscroll bool

	// If Frame is true, Title allows to configure a title for the view.
	Title string

	Tabs     []string
	TabIndex int

	// If Frame is true, Subtitle allows to configure a subtitle for the view.
	Subtitle string

	// If Mask is true, the View will display the mask instead of the real
	// content
	Mask rune

	// Overlaps describes which edges are overlapping with another view's edges
	Overlaps byte

	// If HasLoader is true, the message will be appended with a spinning loader animation
	HasLoader bool

	writeMutex sync.Mutex

	// IgnoreCarriageReturns tells us whether to ignore '\r' characters
	IgnoreCarriageReturns bool

	// ParentView is the view which catches events bubbled up from the given view if there's no matching handler
	ParentView *View

	Context string // this is for assigning keybindings to a view only in certain contexts

	log *logrus.Entry

	// Pty tells us whether we have a pty running within the view. When Pty is set,
	// we will catch keybindings set on the view, but all other keypresses (including
	// those for which there are global keybindings) will be forwarded to the
	// underlying pty as the original byte string. When Pty is set, we will also
	// directly redraw the view when it is written to
	Pty bool

	// StdinWriter is used in conjunction with the Pty flag. When using a pty,
	// any termbox events not caught by the view will be written to this writer
	// as the original byte slice.
	StdinWriter io.Writer
}

type viewLine struct {
	linesX, linesY int // coordinates relative to v.lines
	line           []cell
}

type cell struct {
	chr              rune
	bgColor, fgColor Attribute
}

type lineType []cell

// String returns a string from a given cell slice.
func (l lineType) String() string {
	str := ""
	for _, c := range l {
		str += string(c.chr)
	}
	return str
}

// newView returns a new View object.
func newView(name string, x0, y0, x1, y1 int, mode OutputMode, log *logrus.Entry) *View {
	v := &View{
		name:    name,
		x0:      x0,
		y0:      y0,
		x1:      x1,
		y1:      y1,
		Frame:   true,
		Editor:  DefaultEditor,
		tainted: true,
		ei:      newEscapeInterpreter(mode),
		log:     log,
	}
	return v
}

// Dimensions returns the dimensions of the View
func (v *View) Dimensions() (int, int, int, int) {
	return v.x0, v.y0, v.x1, v.y1
}

// Size returns the number of visible columns and rows in the View.
func (v *View) Size() (x, y int) {
	return v.x1 - v.x0 - 1, v.y1 - v.y0 - 1
}

// Name returns the name of the view.
func (v *View) Name() string {
	return v.name
}

// setRune sets a rune at the given point relative to the view. It applies the
// specified colors, taking into account if the cell must be highlighted. Also,
// it checks if the position is valid.
func (v *View) setRune(x, y int, ch rune, fgColor, bgColor Attribute) error {
	maxX, maxY := v.Size()
	if x < 0 || x >= maxX || y < 0 || y >= maxY {
		return errors.New("invalid point")
	}
	var (
		ry, rcy int
		err     error
	)
	if v.Highlight {
		_, ry, err = v.realPosition(x, y)
		if err != nil {
			return err
		}
		_, rcy, err = v.realPosition(v.cx, v.cy)
		if err != nil {
			return err
		}
	}

	if v.Mask != 0 {
		fgColor = v.FgColor
		bgColor = v.BgColor
		ch = v.Mask
	} else if v.Highlight && ry == rcy {
		fgColor = fgColor | AttrBold
	}

	termbox.SetCell(v.x0+x+1, v.y0+y+1, ch,
		termbox.Attribute(fgColor), termbox.Attribute(bgColor))

	return nil
}

// SetCursor sets the cursor position of the view at the given point,
// relative to the view. It checks if the position is valid.
func (v *View) SetCursor(x, y int) error {
	if x < 0 || y < 0 {
		return nil
	}
	v.cx = x
	v.cy = y
	return nil
}

// Cursor returns the cursor position of the view.
func (v *View) Cursor() (x, y int) {
	return v.cx, v.cy
}

// SetOrigin sets the origin position of the view's internal buffer,
// so the buffer starts to be printed from this point, which means that
// it is linked with the origin point of view. It can be used to
// implement Horizontal and Vertical scrolling with just incrementing
// or decrementing ox and oy.
func (v *View) SetOrigin(x, y int) error {
	v.ox = x
	v.oy = y
	return nil
}

// Origin returns the origin position of the view.
func (v *View) Origin() (x, y int) {
	return v.ox, v.oy
}

func (v *View) padCellsForNewCy() {
	if v.cx > len(v.lines[v.cy]) {
		v.lines[v.cy] = append(v.lines[v.cy], make([]cell, v.cx-len(v.lines[v.cy]))...)
	}
}

func (v *View) moveCursorHorizontally(n int) {
	if n > 0 {
		v.moveCursorRight(n)
	}
	v.moveCursorLeft(-n)
}

func (v *View) moveCursorVertically(n int) {
	if n > 0 {
		v.moveCursorDown(n)
	}
	v.moveCursorUp(-n)
}

func (v *View) moveCursorRight(n int) {
	for i := 0; i < n; i++ {
		if v.cx == len(v.lines[v.cy]) {
			v.lines[v.cy] = append(v.lines[v.cy], cell{})
		}
		v.cx++
	}
}

func (v *View) moveCursorLeft(n int) {
	if v.cx-n <= 0 {
		v.cx = 0
	} else {
		v.cx -= n
	}
}

func (v *View) moveCursorDown(n int) {
	for i := 0; i < n; i++ {
		if v.cy == len(v.lines)-1 {
			v.lines = append(v.lines, nil)
		}
		v.cy++
	}
	v.padCellsForNewCy()
}

func (v *View) moveCursorUp(n int) {
	if v.cy-n <= 0 {
		v.cy = 0
	} else {
		v.cy -= n
	}
	v.padCellsForNewCy()
}

// Write appends a byte slice into the view's internal buffer. Because
// View implements the io.Writer interface, it can be passed as parameter
// of functions like fmt.Fprintf, fmt.Fprintln, io.Copy, etc. Clear must
// be called to clear the view's buffer.
func (v *View) Write(p []byte) (n int, err error) {
	v.tainted = true
	v.writeMutex.Lock()
	defer v.writeMutex.Unlock()

	if len(v.lines) == 0 {
		v.lines = make([][]cell, 1) // not sure why we're doing that
	}

	sanityCheck := func() {
		if v.cx > len(v.lines[v.cy]) {
			v.log.Warn("FAILED! y: ", v.cy, ", x: ", v.cx, ", line length: ", len(v.lines[v.cy]))
			panic("cx too big")
		}
	}

	for _, ch := range bytes.Runes(p) {
		switch ch {
		case '\n':
			if v.cy == len(v.lines)-1 {
				v.lines = append(v.lines, nil)
			}
			v.cy += 1
			v.cx = 0
			sanityCheck()
		case '\r':
			if v.IgnoreCarriageReturns {
				continue
			}
			v.cx = 0
			sanityCheck()
		default:
			cells := v.parseInput(ch)
			if v.ei.instruction.kind != NONE {
				switch v.ei.instruction.kind {
				case CURSOR_UP:
					v.log.Warn("moving cursor up")
					toMoveUp := v.ei.instruction.param1
					v.moveCursorUp(toMoveUp)
					sanityCheck()
				case CURSOR_DOWN:
					v.log.Warn("moving cursor up")
					toMoveDown := v.ei.instruction.param1
					v.moveCursorDown(toMoveDown)
					sanityCheck()

				case CURSOR_LEFT:
					toMoveLeft := v.ei.instruction.param1
					v.log.Warn("moving cursor left: ", toMoveLeft)
					v.moveCursorLeft(toMoveLeft)
					sanityCheck()
				case CURSOR_RIGHT:
					v.log.Warn("moving cursor right")
					toMoveRight := v.ei.instruction.param1
					v.moveCursorRight(toMoveRight)
					sanityCheck()
				case CURSOR_MOVE:
					x := v.ei.instruction.param1
					y := v.ei.instruction.param2
					v.log.Warn("x: ", x)
					v.log.Warn("y: ", y)
					v.moveCursorVertically(y - v.cy)
					v.log.Warn("after moving vertically: y: ", v.cy, ", x: ", v.cx, ", line length: ", len(v.lines[v.cy]))
					v.moveCursorHorizontally(x - v.cx)
					v.log.Warn("after moving horizontally: y: ", v.cy, ", x: ", v.cx, ", line length: ", len(v.lines[v.cy]))
					sanityCheck()
				case ERASE_IN_LINE:
					code := v.ei.instruction.param1
					// need to check if we should delete the character at the cursor as well. I will assume that we don't (cos it's easier code-wise)
					switch code {
					case 0:
						v.log.Warn("line length:", len(v.lines[v.cy]))
						v.log.Warn("cx:", v.cx)
						v.lines[v.cy] = v.lines[v.cy][0:v.cx]
						sanityCheck()
					case 1:
						v.lines[v.cy] = append(make([]cell, v.cx), v.lines[v.cy][v.cx:]...)
						sanityCheck()
					case 2:
						// need to clear line but retain cursor x position
						// so we'll pad everything out to the left
						v.lines[v.cy] = make([]cell, v.cx+1)
						sanityCheck()
					}

				case CLEAR_SCREEN:
					v.log.Warn("clearing screen")
					v.lines = make([][]cell, 1)
					v.cx = 0
					v.cy = 0
					sanityCheck()
				case INSERT_CHARACTER:
					v.log.Warn("inserting character")
					toInsert := v.ei.instruction.param1
					if toInsert == 0 {
						toInsert = 1
					}
					v.lines[v.cy] = append(v.lines[v.cy][:v.cx], append(make([]cell, toInsert), v.lines[v.cy][v.cx:]...)...)
				case DELETE:
					v.log.Warn("deleting characters")
					toDelete := v.ei.instruction.param1
					if toDelete == 0 {
						toDelete = 1
					}
					if v.cx+toDelete > len(v.lines[v.cy]) {
						toDelete = len(v.lines[v.cy]) - v.cx
					}
					v.lines[v.cy] = append(v.lines[v.cy][:v.cx], v.lines[v.cy][v.cx+toDelete:]...)
				default:
					panic("instruction not understood")
				}
				v.ei.instructionRead()
				continue
			}
			if cells == nil {
				continue
			}

			for _, cell := range cells {
				v.log.Warn("y: ", v.cy, ", x: ", v.cx, ", line length: ", len(v.lines[v.cy]), ", cell ch: ", cell.chr)
				if cell.chr == 7 {
					// bell: can't do anything
					continue
				}
				if cell.chr == '\b' {
					if v.cx > 0 {
						v.cx--
					}
					continue
				}
				if v.cx == len(v.lines[v.cy]) {
					v.lines[v.cy] = append(v.lines[v.cy], cell)
				} else if v.cx < len(v.lines[v.cy]) {
					v.lines[v.cy][v.cx] = cell
				} else {
					// TODO: decide whether this matters
					panic(v.name + ": above length for some reason")
				}

				v.cx++
			}
			sanityCheck()
		}
	}

	return len(p), nil
}

// parseInput parses char by char the input written to the View. It returns nil
// while processing ESC sequences. Otherwise, it returns a cell slice that
// contains the processed data.
func (v *View) parseInput(ch rune) []cell {
	cells := []cell{}

	isEscape, err := v.ei.parseOne(ch)
	if err != nil {
		// there is an error parsing an escape sequence, ouput all the escape characters so far as a string
		v.log.Warn(spew.Sdump(v.ei.runes()[1:]))
		panic(string(v.ei.runes()[1:]))
		for _, r := range v.ei.runes() {
			c := cell{
				fgColor: v.FgColor,
				bgColor: v.BgColor,
				chr:     r,
			}
			cells = append(cells, c)
		}
		v.ei.reset()
	} else {
		if isEscape {
			return nil
		}
		repeatCount := 1
		if ch == '\t' {
			ch = ' '
			repeatCount = 4
		}
		for i := 0; i < repeatCount; i++ {
			c := cell{
				fgColor: v.ei.curFgColor,
				bgColor: v.ei.curBgColor,
				chr:     ch,
			}
			cells = append(cells, c)
		}
	}

	return cells
}

// Read reads data into p. It returns the number of bytes read into p.
// At EOF, err will be io.EOF. Calling Read() after Rewind() makes the
// cache to be refreshed with the contents of the view.
func (v *View) Read(p []byte) (n int, err error) {
	if v.readOffset == 0 {
		v.readCache = v.Buffer()
	}
	if v.readOffset < len(v.readCache) {
		n = copy(p, v.readCache[v.readOffset:])
		v.readOffset += n
	} else {
		err = io.EOF
	}
	return
}

// Rewind sets the offset for the next Read to 0, which also refresh the
// read cache.
func (v *View) Rewind() {
	v.readOffset = 0
}

// draw re-draws the view's contents. It returns an array of wrap counts, that is,
// when wrapping is turned on, an array where for each index and value i, v,
// i is the position of a line in the buffer, and v is the number times the line wrapped
func (v *View) draw() ([]int, error) {
	maxX, maxY := v.Size()

	if v.Wrap {
		if maxX == 0 {
			return nil, errors.New("X size of the view cannot be 0")
		}
		v.ox = 0
	}

	wrapCounts := make([]int, len(v.lines))

	if v.tainted {
		v.viewLines = nil
		lines := v.lines
		if v.HasLoader {
			lines = v.loaderLines()
		}
		for i, line := range lines {
			wrap := 0
			if v.Wrap {
				wrap = maxX
			}

			ls := lineWrap(line, wrap)
			wrapCounts[i] = len(ls) - 1
			for j := range ls {
				vline := viewLine{linesX: j, linesY: i, line: ls[j]}
				v.viewLines = append(v.viewLines, vline)
			}
		}
		if !v.HasLoader {
			v.tainted = false
		}
	}

	if v.Autoscroll && len(v.viewLines) > maxY {
		v.oy = len(v.viewLines) - maxY
	}
	y := 0
	for i, vline := range v.viewLines {
		if i < v.oy {
			continue
		}
		if y >= maxY {
			break
		}
		x := 0
		for j, c := range vline.line {
			if j < v.ox {
				continue
			}
			if x >= maxX {
				break
			}

			fgColor := c.fgColor
			if fgColor == ColorDefault {
				fgColor = v.FgColor
			}
			bgColor := c.bgColor
			if bgColor == ColorDefault {
				bgColor = v.BgColor
			}
			if err := v.setRune(x, y, c.chr, fgColor, bgColor); err != nil {
				return nil, err
			}
			x += runewidth.RuneWidth(c.chr)
		}
		y++
	}
	return wrapCounts, nil
}

// realPosition returns the position in the internal buffer corresponding to the
// point (x, y) of the view.
func (v *View) realPosition(vx, vy int) (x, y int, err error) {
	vx = v.ox + vx
	vy = v.oy + vy

	if vx < 0 || vy < 0 {
		return 0, 0, errors.New("invalid point")
	}

	if len(v.viewLines) == 0 {
		return vx, vy, nil
	}

	if vy < len(v.viewLines) {
		vline := v.viewLines[vy]
		x = vline.linesX + vx
		y = vline.linesY
	} else {
		vline := v.viewLines[len(v.viewLines)-1]
		x = vx
		y = vline.linesY + vy - len(v.viewLines) + 1
	}

	return x, y, nil
}

// Clear empties the view's internal buffer.
func (v *View) Clear() {
	v.writeMutex.Lock()
	defer v.writeMutex.Unlock()

	v.tainted = true
	v.ei.reset()

	v.lines = nil
	v.cy = 0
	v.cx = 0
	v.viewLines = nil
	v.readOffset = 0
	v.clearRunes()
}

// clearRunes erases all the cells in the view.
func (v *View) clearRunes() {
	maxX, maxY := v.Size()
	for x := 0; x < maxX; x++ {
		for y := 0; y < maxY; y++ {
			termbox.SetCell(v.x0+x+1, v.y0+y+1, ' ',
				termbox.Attribute(v.FgColor), termbox.Attribute(v.BgColor))
		}
	}
}

// BufferLines returns the lines in the view's internal
// buffer.
func (v *View) BufferLines() []string {
	v.writeMutex.Lock()
	defer v.writeMutex.Unlock()
	lines := make([]string, len(v.lines))
	for i, l := range v.lines {
		str := lineType(l).String()
		str = strings.Replace(str, "\x00", " ", -1)
		lines[i] = str
	}
	return lines
}

// Buffer returns a string with the contents of the view's internal
// buffer.
func (v *View) Buffer() string {
	return linesToString(v.lines)
}

// ViewBufferLines returns the lines in the view's internal
// buffer that is shown to the user.
func (v *View) ViewBufferLines() []string {
	v.writeMutex.Lock()
	defer v.writeMutex.Unlock()
	lines := make([]string, len(v.viewLines))
	for i, l := range v.viewLines {
		str := lineType(l.line).String()
		str = strings.Replace(str, "\x00", " ", -1)
		lines[i] = str
	}
	return lines
}

// LinesHeight is the count of view lines (i.e. lines excluding wrapping)
func (v *View) LinesHeight() int {
	return len(v.lines)
}

// ViewLinesHeight is the count of view lines (i.e. lines including wrapping)
func (v *View) ViewLinesHeight() int {
	return len(v.viewLines)
}

// ViewBuffer returns a string with the contents of the view's buffer that is
// shown to the user.
func (v *View) ViewBuffer() string {
	lines := make([][]cell, len(v.viewLines))
	for i := range v.viewLines {
		lines[i] = v.viewLines[i].line
	}

	return linesToString(lines)
}

// Line returns a string with the line of the view's internal buffer
// at the position corresponding to the point (x, y).
func (v *View) Line(y int) (string, error) {
	_, y, err := v.realPosition(0, y)
	if err != nil {
		return "", err
	}

	if y < 0 || y >= len(v.lines) {
		return "", errors.New("invalid point")
	}

	return lineType(v.lines[y]).String(), nil
}

// Word returns a string with the word of the view's internal buffer
// at the position corresponding to the point (x, y).
func (v *View) Word(x, y int) (string, error) {
	x, y, err := v.realPosition(x, y)
	if err != nil {
		return "", err
	}

	if x < 0 || y < 0 || y >= len(v.lines) || x >= len(v.lines[y]) {
		return "", errors.New("invalid point")
	}

	str := lineType(v.lines[y]).String()

	nl := strings.LastIndexFunc(str[:x], indexFunc)
	if nl == -1 {
		nl = 0
	} else {
		nl = nl + 1
	}
	nr := strings.IndexFunc(str[x:], indexFunc)
	if nr == -1 {
		nr = len(str)
	} else {
		nr = nr + x
	}
	return string(str[nl:nr]), nil
}

// indexFunc allows to split lines by words taking into account spaces
// and 0.
func indexFunc(r rune) bool {
	return r == ' ' || r == 0
}

func lineWidth(line []cell) (n int) {
	for i := range line {
		n += runewidth.RuneWidth(line[i].chr)
	}

	return
}

func lineWrap(line []cell, columns int) [][]cell {
	if columns == 0 {
		return [][]cell{line}
	}

	var n int
	var offset int
	lines := make([][]cell, 0, 1)
	for i := range line {
		rw := runewidth.RuneWidth(line[i].chr)
		n += rw
		if n > columns {
			n = rw
			lines = append(lines, line[offset:i])
			offset = i
		}
	}

	lines = append(lines, line[offset:])
	return lines
}

func linesToString(lines [][]cell) string {
	str := make([]string, len(lines))
	for i := range lines {
		rns := make([]rune, 0, len(lines[i]))
		line := lineType(lines[i]).String()
		for _, c := range line {
			if c != '\x00' {
				rns = append(rns, c)
			}
		}
		str[i] = string(rns)
	}

	return strings.Join(str, "\n")
}

func (v *View) loaderLines() [][]cell {
	duplicate := make([][]cell, len(v.lines))
	for i := range v.lines {
		if i < len(v.lines)-1 {
			duplicate[i] = make([]cell, len(v.lines[i]))
			copy(duplicate[i], v.lines[i])
		} else {
			duplicate[i] = make([]cell, len(v.lines[i])+2)
			copy(duplicate[i], v.lines[i])
			duplicate[i][len(duplicate[i])-2] = cell{chr: ' '}
			duplicate[i][len(duplicate[i])-1] = Loader()
		}
	}

	return duplicate
}

func Loader() cell {
	characters := "|/-\\"
	now := time.Now()
	nanos := now.UnixNano()
	index := nanos / 50000000 % int64(len(characters))
	str := characters[index : index+1]
	chr := []rune(str)[0]
	return cell{
		chr: chr,
	}
}

// IsTainted tells us if the view is tainted
func (v *View) IsTainted() bool {
	return v.tainted
}

// GetClickedTabIndex tells us which tab was clicked
func (v *View) GetClickedTabIndex(x int) int {
	if len(v.Tabs) <= 1 {
		return 0
	}

	charIndex := 0
	for i, tab := range v.Tabs {
		charIndex += len(tab + " - ")
		if x < charIndex {
			return i
		}
	}

	return 0
}

func (v *View) SelectedLineIdx() int {
	_, seletedLineIdx := v.SelectedPoint()
	return seletedLineIdx
}

func (v *View) SelectedPoint() (int, int) {
	cx, cy := v.Cursor()
	ox, oy := v.Origin()
	return cx + ox, cy + oy
}
