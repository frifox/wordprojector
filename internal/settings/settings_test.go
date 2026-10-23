package settings

import (
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"wordprojector/internal/db"
)

func TestRoundTrip(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "wordprojector.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	s := Default()
	s.General.CurrentThemeID = 3
	s.General.DisplayScreen2 = 1
	s.Bible[0].Primary = "2"
	s.App.Window = WindowGeometry{10, 20, 300, 400}
	if err := Save(d, s); err != nil {
		t.Fatal(err)
	}
	s2, err := Load(d)
	if err != nil {
		t.Fatal(err)
	}
	if s2.General.CurrentThemeID != 3 || s2.General.DisplayScreen2 != 1 || s2.Bible[0].Primary != "2" || s2.App.Window.Width != 300 || s2.Bible[1].Operator != "same" {
		t.Fatalf("%+v", s2)
	}
}

// fill gives every exported field of v a value that is not the zero value (and differs per field), recursively.
func fill(v reflect.Value, n *int) {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				fill(v.Field(i), n)
			}
		}
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			fill(v.Index(i), n)
		}
	case reflect.Slice:
		s := reflect.MakeSlice(v.Type(), 2, 2)
		for i := 0; i < 2; i++ {
			fill(s.Index(i), n)
		}
		v.Set(s)
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		k, e := reflect.New(v.Type().Key()).Elem(), reflect.New(v.Type().Elem()).Elem()
		fill(k, n)
		fill(e, n)
		m.SetMapIndex(k, e)
		v.Set(m)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int64, reflect.Int32:
		*n++
		v.SetInt(int64(*n%90 + 7))
	case reflect.Float64:
		*n++
		v.SetFloat(float64(*n%50) + 0.5)
	case reflect.String:
		*n++
		v.SetString("v" + strconv.Itoa(*n))
	}
}

// Every setting the program has must come back after a save and a load (a restart): a field that does not is a
// preference that is forgotten.
func TestEverySettingIsSaved(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "wordprojector.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	var s Settings
	n := 0
	fill(reflect.ValueOf(&s).Elem(), &n)
	s.App.Transition.FadeMillis = 700 // a fade time outside 50..5000 is not kept
	for i := range s.Bible {
		s.Bible[i].Operator = "same" // the old "operator Bible" is no longer used
	}
	if err := Save(d, s); err != nil {
		t.Fatal(err)
	}
	got, err := Load(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []struct {
		name      string
		want, got any
	}{{"General", s.General, got.General}, {"Main", s.Main, got.Main}, {"Bible", s.Bible, got.Bible}, {"Slides", s.Slides, got.Slides}, {"App", s.App, got.App}} {
		w, g := reflect.ValueOf(part.want), reflect.ValueOf(part.got)
		compare(t, part.name, w, g)
	}
}

func compare(t *testing.T, path string, w, g reflect.Value) {
	switch w.Kind() {
	case reflect.Struct:
		for i := 0; i < w.NumField(); i++ {
			if w.Type().Field(i).IsExported() {
				compare(t, path+"."+w.Type().Field(i).Name, w.Field(i), g.Field(i))
			}
		}
	case reflect.Array, reflect.Slice:
		if w.Len() != g.Len() {
			t.Errorf("%s: %d values saved, %d came back", path, w.Len(), g.Len())
			return
		}
		for i := 0; i < w.Len(); i++ {
			compare(t, path+"["+strconv.Itoa(i)+"]", w.Index(i), g.Index(i))
		}
	default:
		if !reflect.DeepEqual(w.Interface(), g.Interface()) {
			t.Errorf("%s is not saved: wanted %v, got %v", path, w.Interface(), g.Interface())
		}
	}
}
