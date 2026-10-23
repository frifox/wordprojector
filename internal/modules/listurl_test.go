package modules

import "testing"

func TestListURL(t *testing.T) {
	cases := []struct{ kind, src, custom, want string; err bool }{
		{"bible", "softprojector", "", "http://softprojector.org/bibles/bible.xml", false},
		{"songbook", "", "", "http://softprojector.org/songbooks/songbooks.xml", false},
		{"bible", "wordprojector", "", "", true},
		{"theme", "custom", "example.org/", "http://example.org/themes/themes.xml", false},
		{"bible", "custom", "https://x.org/files/my.xml", "https://x.org/files/my.xml", false},
		{"bible", "custom", "", "", true},
		{"bible", "custom", "ftp://x.org", "", true},
		{"nope", "softprojector", "", "", true},
		{"bible", "other", "", "", true},
	}
	for _, c := range cases {
		got, err := ListURL(c.kind, c.src, c.custom)
		if (err != nil) != c.err || got != c.want {
			t.Errorf("ListURL(%q,%q,%q) = %q, %v; want %q (err %v)", c.kind, c.src, c.custom, got, err, c.want, c.err)
		}
	}
}
