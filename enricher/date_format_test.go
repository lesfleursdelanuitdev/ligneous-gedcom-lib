package enricher

import "testing"

func TestFormatGEDCOMDate_June2016(t *testing.T) {
	pd := ParseDateString("June 2016")
	got := FormatGEDCOMDate(pd)
	if got != "JUN 2016" {
		t.Fatalf("FormatGEDCOMDate(June 2016): want JUN 2016, got %q", got)
	}
}

func TestFormatGEDCOMDate_Empty(t *testing.T) {
	pd := ParseDateString("")
	if FormatGEDCOMDate(pd) != "" {
		t.Fatalf("expected empty string, got %q", FormatGEDCOMDate(pd))
	}
}

func TestFormatGEDCOMDate_AboutMonthYear(t *testing.T) {
	pd := ParseDateString("abt June 2016")
	got := FormatGEDCOMDate(pd)
	if got != "ABT JUN 2016" {
		t.Fatalf("want ABT JUN 2016, got %q", got)
	}
}

func TestFormatGEDCOMDate_Between(t *testing.T) {
	pd := ParseDateString("bet 1 jan 1900 and 31 dec 1950")
	got := FormatGEDCOMDate(pd)
	want := "BET 1 JAN 1900 AND 31 DEC 1950"
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}
