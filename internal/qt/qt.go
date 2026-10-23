// Package qt converts between the Qt-specific encodings used by the original
// SoftProjector database (QFont::toString, QColor::rgb, QVariant booleans)
// and plain Go values, so the same spData.sqlite stays readable by both apps.
package qt

import (
	"fmt"
	"strconv"
	"strings"
)

// Font is the subset of QFont that the projector actually uses.
type Font struct {
	Family    string  `json:"family"`
	PointSize float64 `json:"pointSize"`
	Bold      bool    `json:"bold"`
	Italic    bool    `json:"italic"`
	Underline bool    `json:"underline"`
	Strikeout bool    `json:"strikeout"`
	// raw keeps the original tail of the QFont string so that round-tripping
	// a font we did not change preserves every Qt field.
	raw []string
}

// DefaultFont mirrors the defaults used throughout the original code.
func DefaultFont(size float64) Font {
	return Font{Family: "Arial", PointSize: size}
}

// ParseFont decodes a QFont::toString() value such as
// "Arial,28,-1,5,50,0,0,0,0,0" (Qt5, 10 fields) or the longer Qt6 form
// "Arial,28,-1,5,400,0,0,0,0,0,0,0,0,0,0,1" (16+ fields).
func ParseFont(s string) (Font, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Font{}, false
	}
	parts := strings.Split(s, ",")
	if len(parts) < 2 {
		return Font{Family: parts[0]}, parts[0] != ""
	}
	f := Font{Family: parts[0], raw: parts}
	f.PointSize, _ = strconv.ParseFloat(parts[1], 64)
	if f.PointSize <= 0 && len(parts) > 2 {
		// pixel size may be given instead of point size
		px, _ := strconv.ParseFloat(parts[2], 64)
		if px > 0 {
			f.PointSize = px * 72 / 96
		}
	}
	if len(parts) > 4 {
		w, _ := strconv.Atoi(parts[4])
		// Qt5 weights: 50 normal, 75 bold. Qt6 weights: 400 normal, 700 bold.
		if w >= 100 {
			f.Bold = w >= 600
		} else {
			f.Bold = w >= 63
		}
	}
	if len(parts) > 5 {
		f.Italic = parts[5] == "1"
	}
	if len(parts) > 6 {
		f.Underline = parts[6] == "1"
	}
	if len(parts) > 7 {
		f.Strikeout = parts[7] == "1"
	}
	// Qt6 appends the style name as the last field ("Bold", "Italic", ...).
	if len(parts) >= 16 {
		style := strings.ToLower(parts[len(parts)-1])
		if strings.Contains(style, "bold") {
			f.Bold = true
		}
		if strings.Contains(style, "italic") || strings.Contains(style, "oblique") {
			f.Italic = true
		}
	}
	return f, true
}

// String encodes the font in the Qt5 10-field QFont::toString format, which
// both Qt5 and Qt6 builds of SoftProjector can read back.
func (f Font) String() string {
	family := f.Family
	if family == "" {
		family = "Arial"
	}
	size := f.PointSize
	if size <= 0 {
		size = 16
	}
	weight := 50
	if f.Bold {
		weight = 75
	}
	b := func(v bool) string {
		if v {
			return "1"
		}
		return "0"
	}
	return fmt.Sprintf("%s,%s,-1,5,%d,%s,%s,%s,0,0", family, trimFloat(size), weight, b(f.Italic), b(f.Underline), b(f.Strikeout))
}

func trimFloat(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// Color helpers ------------------------------------------------------------

// RGBToHex converts a QColor::rgb() value (0xAARRGGBB stored as unsigned int)
// into a "#rrggbb" string. Alpha is ignored like the original app does.
func RGBToHex(v int64) string {
	u := uint32(v)
	return fmt.Sprintf("#%02x%02x%02x", (u>>16)&0xff, (u>>8)&0xff, u&0xff)
}

// HexToRGB converts "#rrggbb" (or "#rgb") into the QColor::rgb() integer with
// alpha set to 0xff, which is what Qt writes.
func HexToRGB(hex string) int64 {
	hex = strings.TrimSpace(strings.TrimPrefix(hex, "#"))
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return int64(0xffffffff)
	}
	n, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return int64(0xffffffff)
	}
	return int64(0xff000000 | uint32(n))
}

// ColorOrDefault returns the hex color for a nullable integer column.
func ColorOrDefault(v *int64, def string) string {
	if v == nil {
		return def
	}
	return RGBToHex(*v)
}

// ParseBool accepts the many spellings Qt may have written: 1/0, "true"/"false".
func ParseBool(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case int64:
		return t != 0
	case int:
		return t != 0
	case float64:
		return t != 0
	case []byte:
		return parseBoolString(string(t))
	case string:
		return parseBoolString(t)
	}
	return false
}

func parseBoolString(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "1" || s == "true" || s == "yes"
}

// ParseInt accepts text or numeric sqlite values.
func ParseInt(v any) (int64, bool) {
	switch t := v.(type) {
	case nil:
		return 0, false
	case int64:
		return t, true
	case int:
		return int64(t), true
	case float64:
		return int64(t), true
	case []byte:
		n, err := strconv.ParseInt(strings.TrimSpace(string(t)), 10, 64)
		return n, err == nil
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return n, err == nil
	}
	return 0, false
}

// ParseString accepts text or blob sqlite values.
func ParseString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}
