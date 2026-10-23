package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestModeFor(t *testing.T) {
	cases := []struct {
		name            string
		tty, json, nocl bool
		env             string
		want            Mode
	}{
		{"tty", true, false, false, "", Styled},
		{"json", true, true, false, "", JSON},
		{"json wins over nocolor", true, true, true, "1", JSON},
		{"json no tty", false, true, false, "", JSON},
		{"no tty", false, false, false, "", Plain},
		{"NO_COLOR env", true, false, false, "1", Plain},
		{"--no-color", true, false, true, "", Plain},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ModeFor(c.tty, c.json, c.nocl, func(string) string { return c.env })
			if got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestPlainHasNoANSI(t *testing.T) {
	var b bytes.Buffer
	p := Printer{W: &b, Mode: Plain}
	p.Ok("Git installed")
	if b.String() != "✓ Git installed\n" {
		t.Fatalf("got %q", b.String())
	}
	if strings.Contains(b.String(), "\x1b[") {
		t.Fatal("ANSI in plain output")
	}
}

func TestWarnFailSymbols(t *testing.T) {
	var b bytes.Buffer
	p := Printer{W: &b, Mode: Plain}
	p.Warn("x")
	p.Fail("x")
	if b.String() != "! x\n✗ x\n" {
		t.Fatalf("got %q", b.String())
	}
}

func TestZeroPrinterIsPlain(t *testing.T) {
	var b bytes.Buffer
	var m Mode
	if m != Plain {
		t.Fatalf("zero Mode = %v, want Plain", m)
	}
	Printer{W: &b}.Heading("Doctor")
	if strings.Contains(b.String(), "\x1b[") {
		t.Fatal("ANSI from a zero-value Printer")
	}
}

func TestOkStyleGreen(t *testing.T) {
	if got := okStyle.Render("✓"); !strings.Contains(got, "38;2;34;197;94") {
		t.Errorf("okStyle is not green: %q", got)
	}
	var b bytes.Buffer
	Printer{W: &b, Mode: Styled}.Ok("x")
	if !strings.Contains(b.String(), "38;2;34;197;94") {
		t.Errorf("styled Ok is not green: %q", b.String())
	}
	b.Reset()
	Printer{W: &b, Mode: Plain}.Ok("x")
	if b.String() != "✓ x\n" {
		t.Errorf("plain Ok = %q", b.String())
	}
}
