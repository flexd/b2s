package ircbot

import (
	"fmt"
)

// The colors specified by mIRC.
const (
	White       Color = "0"
	Black             = "1"
	Blue              = "2"
	Green             = "3"
	Red               = "4"
	Brown             = "5"
	Purple            = "6"
	Orange            = "7"
	Yellow            = "8"
	LightGreen        = "9"
	Teal              = "10"
	LightCyan         = "11"
	LightBlue         = "12"
	LightPurple       = "13"
	Gray              = "14"
	LightGray         = "15"
	Default           = "99"
)

// The formatting attributes specified by mIRC.
const (
	Bold      Attrib = "\x02"
	Italic           = "\x1D"
	Underline        = "\x1F"
	Video            = "\x16"
	Reset            = "\x0F"
)

// Represents an ongoing text format operation.
type Fmt struct {
	str     string
	fg      Color
	bg      Color
	attribs []Attrib
}

// Represents a mIRC color.
type Color string

// Represents a mIRC text format attribute.
type Attrib string

func (a Attrib) String() string {
	return string(a)
}

func (c Color) value() string {
	if len(c) == 0 {
		return Default
	}
	return c.String()
}

func (c Color) String() string {
	return string(c)
}

// Returns a new format object from a string 's'.
func F(s string) Fmt {
	return Fmt{str: s}
}

// Replaces the foreground color with 'c' in the format.
func (f Fmt) Fg(c Color) Fmt {
	f.fg = c
	return f
}

// Replaces the background color with 'c' in the format.
func (f Fmt) Bg(c Color) Fmt {
	f.bg = c
	return f
}

// Adds a new attribute 'a' to the format.
func (f Fmt) Attr(a Attrib) Fmt {
	f.attribs = append(f.attribs, a)
	return f
}

// Extracts the string of some format.
func (f Fmt) String() string {
	s := fmt.Sprintf("\x03%s,%s%s\x03", f.fg.value(), f.bg.value(), f.str)
	for _, a := range f.attribs {
		if a == Reset {
			s += a.String()
			break
		}
		s = fmt.Sprintf("%s%s%s", a, s, a)
	}
	return s
}
