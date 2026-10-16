package parser

import (
	"testing"

	"golang.org/x/text/unicode/norm"
)

func TestANSELCombining(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want string
		bad  int
	}{
		{"acute", []byte{0xE2, 'e'}, "é", 0},
		{"diaeresis", []byte{0xE8, 'u'}, "ü", 0},
		{"cedilla", []byte{0xF0, 'c'}, "ç", 0},
		{"mark inside text", []byte("Ren\xE2e!"), "René!", 0},
		{"trailing mark dropped", []byte{'a', 0xE2}, "a", 1},
		{"mark before newline dropped", []byte{0xE2, '\n', 'a'}, "\na", 1},
	}
	for _, c := range cases {
		got, bad := decodeANSEL(c.in)
		if got != c.want || bad != c.bad {
			t.Errorf("%s: got %q bad=%d, want %q bad=%d", c.name, got, bad, c.want, c.bad)
		}
	}

	// two marks: circumflex then acute, before the base letter
	got, bad := decodeANSEL([]byte{0xE3, 0xE2, 'a'})
	if bad != 0 {
		t.Errorf("two marks: bad=%d", bad)
	}
	if !norm.NFC.IsNormalString(got) {
		t.Errorf("two marks: %q is not NFC", got)
	}
	want := norm.NFC.String("ấ")
	if got != want {
		t.Errorf("two marks: got %+q want %+q", got, want)
	}
}

func TestANSELSpecials(t *testing.T) {
	cases := map[byte]string{
		0xA5: "Æ", 0xB5: "æ", 0xA2: "Ø", 0xB2: "ø",
		0xA1: "Ł", 0xB1: "ł", 0xCF: "ß", 0xC3: "©", 0xB9: "£",
	}
	for b, want := range cases {
		got, bad := decodeANSEL([]byte{b})
		if got != want || bad != 0 {
			t.Errorf("%#x: got %q bad=%d, want %q", b, got, bad, want)
		}
	}
	got, bad := decodeANSEL([]byte{0xFF})
	if got != "�" || bad != 1 {
		t.Errorf("0xFF: got %q bad=%d", got, bad)
	}
	got, bad = decodeANSEL([]byte("plain ASCII"))
	if got != "plain ASCII" || bad != 0 {
		t.Errorf("ascii: got %q bad=%d", got, bad)
	}
}
