package desktop

import "testing"

func TestParseScreensJSON(t *testing.T) {
	arr := parseScreensJSON([]byte(`[{"name":"A","x":0,"y":0,"width":1920,"height":1080,"isPrimary":true},{"name":"B","x":1920,"y":-100,"width":1280,"height":720,"isPrimary":false}]`))
	if len(arr) != 2 || arr[1].X != 1920 || arr[1].Y != -100 || !arr[0].IsPrimary {
		t.Fatalf("array: %+v", arr)
	}
	one := parseScreensJSON([]byte(`{"name":"\\\\.\\DISPLAY1","x":0,"y":0,"width":1366,"height":768,"isPrimary":true}`))
	if len(one) != 1 || one[0].Width != 1366 {
		t.Fatalf("single object: %+v", one)
	}
	if parseScreensJSON([]byte(`not json`)) != nil {
		t.Fatal("garbage should give nil")
	}
}

func TestDetectScreensRuns(t *testing.T) {
	// must never panic or hang, whatever the machine offers
	for i, s := range DetectScreens() {
		if s.Index != i || s.Width <= 0 || s.Height <= 0 {
			t.Fatalf("bad screen %+v", s)
		}
	}
}
