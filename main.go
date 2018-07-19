package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"sync"

	"github.com/mattn/go-runewidth"

	"github.com/gdamore/tcell"
)

// Input modes
const (
	NormalMode = iota
	SeekInputMode
	PatternInputMode
)

// Actions
const (
	BinxResize         = "BINX_RESIZE"
	BinxEscape         = "BINX_ESCAPE"
	BinxKeyEnter       = "BINX_KEYENTER"
	BinxKeyUp          = "BINX_KEYUP"
	BinxKeyDown        = "BINX_KEYDOWN"
	BinxSetScreenStyle = "BINX_SETSTYLE"
	BinxToggleSeekMode = "BINX_TOGGLE_SEEKMODE"
	BinxKeyS           = "BINX_KEY_S"
	BinxKeyF           = "BINX_KEY_F"
	BinxKeyOther       = "BINX_KEY_OTHER"
)

// binxConfig holds binx configuration data
type binxConfig struct {
	filename        string
	screen          tcell.Screen
	byteVisWidth    int
	byteVisHeight   int
	statStyle       tcell.Style
	byteStyle       tcell.Style
	alertStyle      tcell.Style
	startByte       int64
	mode            int
	userInput       string
	highlightPos    int64
	highlightLength int64
}

type AppState struct {
	mutex           sync.Mutex
	filename        string
	dat             []byte
	screen          tcell.Screen
	byteVisWidth    int
	byteVisHeight   int
	statStyle       tcell.Style
	byteStyle       tcell.Style
	alertStyle      tcell.Style
	startByte       int64
	mode            int
	userInput       string
	highlightPos    int64
	highlightLength int64
	lastAction      string
	status          string
}

type Action struct {
	name  string
	value interface{}
}

type Store struct {
	Dispatcher chan Action
	reducer    func(Action) *AppState
	state      *AppState
}

func CreateStore(rootReducer func(Action) *AppState) Store {
	return Store{reducer: rootReducer, Dispatcher: make(chan Action, 20)}
}

// Reduce waits for events on the dispatcher channel then runs them
// through the user-defined reducer to update app state.
func (s *Store) Reduce() {
	for {
		select {
		case action := <-s.Dispatcher:
			s.reducer(action)
		}
	}
}

// HandleTcellEvent translates tcell UI events into Actions
// that can be consumed by our state reducer.
func HandleTcellEvent(store Store, ev tcell.Event) {
	switch ev := ev.(type) {
	case *tcell.EventResize:
		store.Dispatcher <- Action{name: BinxResize, value: ev}
	case *tcell.EventKey:
		if ev.Key() == tcell.KeyEscape {
			store.Dispatcher <- Action{name: BinxEscape, value: ev}
		} else if ev.Key() == tcell.KeyDown {
			store.Dispatcher <- Action{name: BinxKeyDown, value: ev}
		} else if ev.Key() == tcell.KeyUp {
			store.Dispatcher <- Action{name: BinxKeyUp, value: ev}
		} else if ev.Key() == tcell.KeyEnter {
			store.Dispatcher <- Action{name: BinxKeyEnter, value: ev}
		} else if ev.Rune() == 's' {
			store.Dispatcher <- Action{name: BinxKeyS, value: ev.Rune()}
		} else if ev.Rune() == 'f' {
			store.Dispatcher <- Action{name: BinxKeyF, value: ev.Rune()}
		} else {
			store.Dispatcher <- Action{name: BinxKeyOther, value: ev.Rune()}
		}
	}
}

// emitStr prepares a string for render. Will appear after Screen.Show()
func emitStr(s tcell.Screen, x, y int, style tcell.Style, str string) {
	for _, c := range str {
		var combiningChars []rune
		width := runewidth.RuneWidth(c)
		if width == 0 {
			combiningChars = []rune{c}
			c = ' '
			width = 1
		}
		s.SetContent(x, y, c, combiningChars, style)
		x += width
	}
}

// emitStatBar renders the status bar
func emitStatBar(state *AppState) {
	w, h := state.screen.Size()
	numVisibleBytes := w * h
	emitStr(state.screen,
		0, h-1, state.statStyle,
		fmt.Sprintf("--%d--%d--%d--Last Action: %s--Status: %s...\t%s", state.startByte, state.startByte+int64(numVisibleBytes), state.mode, state.lastAction, state.status, state.userInput))
}

// findPattern searches buf for a byte pattern specified by the hex
// string p.
func findBytePattern(p string, buf []byte) (int64, error) {
	decodedPattern, err := hex.DecodeString(p)
	if err != nil {
		return 0, err
	}

	loc := bytes.Index(buf, decodedPattern)

	return int64(loc), nil
}

// rootReducer is a closure around the state object. It returns
// a function that takes an Action. That is the function we pass
// to CreateStore(). rootReducer is run by Reduce() when an event appears
// on the Dispatcher channel. We don't bother with a deep copy of the state
// object; we're just careful with our mutex.
func rootReducer(state *AppState) func(Action) *AppState {
	return func(action Action) *AppState {
		state.mutex.Lock()
		state.lastAction = action.name
		switch action.name {
		case BinxKeyUp:
			w := state.byteVisWidth
			state.startByte -= int64(w)
			if state.startByte < 0 {
				state.startByte = 0
			}
		case BinxKeyDown:
			state.startByte += int64(state.byteVisWidth)
		case BinxResize:
			_, h := state.screen.Size()
			state.byteVisHeight = h - 1
		case BinxEscape:
			if state.mode == NormalMode {
				state.screen.Fini()
				os.Exit(0)
			}
			if state.mode == SeekInputMode {
				state.mode = NormalMode
				state.userInput = ""
			}
			if state.mode == PatternInputMode {
				state.mode = NormalMode
				state.userInput = ""
			}
		case BinxKeyEnter:
			if state.mode == SeekInputMode {
				startByte, err := strconv.ParseInt(state.userInput, 0, 64) // hex, dec, or octal
				state.startByte = startByte
				state.userInput = ""
				state.mode = NormalMode
				if err != nil {
					break
				}
			} else if state.mode == PatternInputMode {
				highlightPos, err := findBytePattern(state.userInput, state.dat)
				if err != nil {
					state.status = err.Error()
					break
				}
				state.highlightPos = highlightPos
				state.status = fmt.Sprintf("%d", highlightPos)
				state.userInput = ""
				state.mode = NormalMode
			}
		case BinxSetScreenStyle:
			state.screen.SetStyle(action.value.(tcell.Style))
		case BinxKeyS:
			if state.mode == NormalMode {
				state.mode = SeekInputMode
				state.userInput = ""
			} else if state.mode == SeekInputMode {
				state.userInput += string(action.value.(rune))
			}
		case BinxKeyF:
			if state.mode == NormalMode {
				state.mode = PatternInputMode
				state.userInput = ""
			} else {
				state.userInput += string(action.value.(rune))
			}
		case BinxKeyOther:
			if state.mode == SeekInputMode {
				state.userInput += string(action.value.(rune))
			}
		default:
			break
		}
		state.mutex.Unlock()
		return state
	}
}

func render(state *AppState) {
	state.screen.Show()
	state.mutex.Lock()
	w := state.byteVisWidth
	h := state.byteVisHeight
	numVisibleBytes := w * h
	if numVisibleBytes < 0 {
		numVisibleBytes = 0
	}
	for i, b := range state.dat[state.startByte : state.startByte+int64(numVisibleBytes)] {
		state.byteStyle = state.byteStyle.Foreground(tcell.Color(b))
		state.screen.SetContent(i%w, i/w, tcell.RuneBoard, nil, state.byteStyle)
	}
	emitStatBar(state)
	state.mutex.Unlock()
	state.screen.Show()
}

func main() {
	filename := flag.String("f", "", "Name of file to view.")
	flag.Parse()

	if *filename == "" {
		fmt.Printf("Filename is required. Use the -f flag.")
		os.Exit(1)
	}

	dat, err := ioutil.ReadFile(*filename)

	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	tcell.SetEncodingFallback(tcell.EncodingFallbackASCII)
	s, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	s.Init()
	_, termHeight := s.Size()
	state := AppState{
		filename:      *filename,
		dat:           dat,
		screen:        s,
		byteVisWidth:  80,
		byteVisHeight: termHeight - 1,
		byteStyle: tcell.StyleDefault.
			Foreground(tcell.ColorLightBlue).
			Background(tcell.ColorBlack),
		statStyle:  tcell.StyleDefault,
		alertStyle: tcell.StyleDefault,
		startByte:  0,
		mode:       NormalMode,
	}

	store := CreateStore(rootReducer(&state))

	// Start the goroutine to receive actions.
	go store.Reduce()

	store.Dispatcher <- Action{name: BinxSetScreenStyle, value: state.byteStyle}

	for {
		ev := s.PollEvent()
		HandleTcellEvent(store, ev)
		render(&state)
	}
}
