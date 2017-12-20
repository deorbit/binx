package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/mattn/go-runewidth"

	"github.com/gdamore/tcell"
)

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

func main() {
	fmt.Println("binx")

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
	style := tcell.StyleDefault.
		Foreground(tcell.ColorLightBlue).
		Background(tcell.ColorBlack)
	s.SetStyle(style)

	w, h := s.Size()
	numChars := w * h

	for {
		s.Show()
		ev := s.PollEvent()
		for i, b := range dat[0:numChars] {
			style = style.Foreground(tcell.Color(b))

			s.SetContent(i%w, i/w, tcell.RuneBoard, nil, style)
			// emitStr(s, 0, 0, style, fmt.Sprintf("--%d--%d--%d chars--", w, h, w*h))
		}
		switch ev := ev.(type) {
		case *tcell.EventResize:
			s.Sync()
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape {
				s.Fini()
				os.Exit(0)
			}
		}
	}
}
