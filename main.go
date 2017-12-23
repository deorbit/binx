package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/mattn/go-runewidth"

	"github.com/gdamore/tcell"
)

// Input modes
const (
	NormalMode = iota
	SeekInputMode
)

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

// binxConfig holds binx configuration data
type binxConfig struct {
	filename      string
	screen        tcell.Screen
	byteVisWidth  int
	byteVisHeight int
	statStyle     tcell.Style
	byteStyle     tcell.Style
	alertStyle    tcell.Style
	startByte     int64
	mode          int
	userInput     string
}

// emitStatBar renders the status bar
func emitStatBar(conf binxConfig) {
	w, h := conf.screen.Size()
	numVisibleBytes := w * h
	emitStr(conf.screen,
		0, h-1, conf.statStyle,
		fmt.Sprintf("--%d--%d--%d chars--", conf.startByte, conf.startByte+int64(numVisibleBytes), numVisibleBytes))
}

func main() {
	filename := flag.String("f", "", "Name of file to view.")
	flag.Parse()

	if *filename == "" {
		fmt.Printf("Filename is required. Use the -f flag.")
		os.Exit(1)
	}

	conf := binxConfig{filename: *filename}

	dat, err := ioutil.ReadFile(conf.filename)

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

	conf.screen = s
	conf.screen.Init()
	_, termHeight := s.Size()
	conf.byteVisWidth = 80
	conf.byteVisHeight = termHeight - 1
	conf.byteStyle = tcell.StyleDefault.
		Foreground(tcell.ColorLightBlue).
		Background(tcell.ColorBlack)
	conf.statStyle = tcell.StyleDefault
	conf.alertStyle = tcell.StyleDefault
	conf.screen.SetStyle(conf.byteStyle)

	conf.startByte = 0

	for {
		_, termHeight = conf.screen.Size()
		w := conf.byteVisWidth
		h := conf.byteVisHeight
		numVisibleBytes := w * h
		for i, b := range dat[conf.startByte : conf.startByte+int64(numVisibleBytes)] {
			conf.byteStyle = conf.byteStyle.Foreground(tcell.Color(b))

			s.SetContent(i%w, i/w, tcell.RuneBoard, nil, conf.byteStyle)
		}

		emitStatBar(conf)

		// Do we have the byte seeker text input open?
		if conf.mode == SeekInputMode {
			emitStr(conf.screen, 0, 0, conf.alertStyle, "SEEK INPUT MODE")
			emitStr(conf.screen,
				conf.byteVisWidth-10,
				conf.byteVisHeight-1,
				tcell.StyleDefault,
				fmt.Sprintf("Jump to: %s", conf.userInput),
			)
		}
		s.Show()

		// Input handling.
		ev := s.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			w, h = s.Size()
			conf.byteVisHeight = h - 1
			numVisibleBytes = conf.byteVisHeight * conf.byteVisWidth
			s.Sync()
		case *tcell.EventKey:
			if conf.mode == SeekInputMode { // Text input mode
				if ev.Key() == tcell.KeyEscape {
					conf.mode = NormalMode
				} else if ev.Key() == tcell.KeyEnter {
					conf.startByte, err = strconv.ParseInt(conf.userInput, 0, 64) // hex, dec, or octal
					conf.userInput = ""
					conf.mode = NormalMode
					if err != nil {
						break
					}
				} else {
					conf.userInput += string(ev.Rune())
				}
			} else {
				// Mouse and arrow key browse mode, aka normal mode.
				if ev.Key() == tcell.KeyEscape {
					s.Fini()
					os.Exit(0)
				} else if ev.Key() == tcell.KeyDown {
					conf.startByte += int64(w)
					s.Sync()
				} else if ev.Key() == tcell.KeyUp {
					conf.startByte -= int64(w)
					if conf.startByte < 0 {
						conf.startByte = 0
					}
					s.Sync()
				} else if ev.Rune() == 's' {
					conf.mode = SeekInputMode
				}
			}
		}
	}
}
