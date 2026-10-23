package qt

import "testing"

func TestFontRoundTrip(t *testing.T) {
	f, ok := ParseFont("Arial,28,-1,5,50,0,0,0,0,0")
	if !ok || f.Family != "Arial" || f.PointSize != 28 || f.Bold {
		t.Fatalf("bad parse: %+v", f)
	}
	if s := f.String(); s != "Arial,28,-1,5,50,0,0,0,0,0" {
		t.Fatalf("bad string: %s", s)
	}
	f2, _ := ParseFont("Times New Roman,36,-1,5,75,1,0,0,0,0")
	if !f2.Bold || !f2.Italic {
		t.Fatalf("expected bold italic: %+v", f2)
	}
	f3, _ := ParseFont("Arial,28,-1,5,700,0,0,0,0,0,0,0,0,0,0,1,Bold")
	if !f3.Bold {
		t.Fatalf("expected qt6 bold: %+v", f3)
	}
}

func TestColors(t *testing.T) {
	if RGBToHex(4294967295) != "#ffffff" {
		t.Fatal("white")
	}
	if HexToRGB("#ffffff") != 4294967295 {
		t.Fatalf("white back: %d", HexToRGB("#ffffff"))
	}
	if RGBToHex(HexToRGB("#12ab34")) != "#12ab34" {
		t.Fatal("roundtrip")
	}
}

func TestBool(t *testing.T) {
	if !ParseBool("true") || ParseBool("false") || !ParseBool(int64(1)) || ParseBool(nil) {
		t.Fatal("bool")
	}
}
