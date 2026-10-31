package songs

import "testing"

func TestLenientInt(t *testing.T) {
	for _, c := range []struct {
		in    any
		want  int64
		valid bool
	}{{int64(7), 7, true}, {"12", 12, true}, {[]byte("3"), 3, true}, {"", 0, false}, {"x", 0, false}, {nil, 0, false}, {2.0, 2, true}} {
		var n lenientInt
		if err := n.Scan(c.in); err != nil || n.Int64 != c.want || n.Valid != c.valid {
			t.Errorf("Scan(%#v) = %v %v %v, want %v %v", c.in, n.Int64, n.Valid, err, c.want, c.valid)
		}
	}
}
