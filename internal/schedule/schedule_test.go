package schedule

import (
	"path/filepath"
	"testing"

	"wordprojector/internal/announce"
	"wordprojector/internal/media"
	"wordprojector/internal/qt"
	"wordprojector/internal/slides"
	"wordprojector/internal/songs"
)

type noBlobs struct{}

func (noBlobs) SongBackground(int64) []byte                { return nil }
func (noBlobs) SlideImages(int64) ([]byte, []byte, []byte) { return nil, nil, nil }

func TestRoundTrip(t *testing.T) {
	s := New()
	s.NewFile()
	s.Add(Item{Type: "bible", Name: "Gen 1:1", Bible: &BibleRef{VerseIDs: "B001C001V001", Caption: "Genesis 1:1", CaptionLong: "Genesis 1:1 In the beginning"}})
	sg := songs.NewSong()
	sg.ID, sg.Number, sg.Title, sg.Text, sg.SongbookName, sg.Font = 5, 12, "Amazing Grace", "Verse 1\nAmazing grace", "Hymns", qt.DefaultFont(40)
	s.Add(Item{Type: "song", Name: "12 Amazing Grace", Song: &sg})
	key := s.SetBlob([]byte("jpegdata"))
	s.Blobs[key+":full"], s.Blobs[key+":small"], s.Blobs[key+":prev"] = []byte("full"), []byte("small"), []byte("prev")
	s.Add(Item{Type: "slideshow", Name: "Pics", Slideshow: &slides.ShowWithSlides{Show: slides.Show{ID: 3, Name: "Pics"}, Slides: []slides.Slide{{Name: "a.jpg", Path: "/x/a.jpg", Temp: key}}}})
	s.Add(Item{Type: "media", Name: "clip.mp4", Media: &media.Item{Path: "/x/clip.mp4", Name: "clip.mp4"}})
	a := announce.New()
	a.Title, a.Text = "Picnic", "Announce\nSaturday"
	s.Add(Item{Type: "announce", Name: "Picnic", Announce: &a})
	s.Move(4, 0)
	path := filepath.Join(t.TempDir(), "t.spsc")
	if err := s.Save(path, noBlobs{}); err != nil {
		t.Fatal(err)
	}
	s2 := New()
	if err := s2.Open(path); err != nil {
		t.Fatal(err)
	}
	items, file, saved := s2.Snapshot()
	if file != path || !saved || len(items) != 5 {
		t.Fatalf("%d items, %s, %v", len(items), file, saved)
	}
	if items[0].Type != "announce" || items[0].Announce.Title != "Picnic" {
		t.Fatalf("order: %+v", items[0])
	}
	if items[2].Song == nil || items[2].Song.Title != "Amazing Grace" || items[2].Song.Font.PointSize != 40 || items[2].Song.Number != 12 {
		t.Fatalf("song: %+v", items[2].Song)
	}
	if items[3].Slideshow == nil || len(items[3].Slideshow.Slides) != 1 || items[3].Slideshow.Slides[0].Temp == "" {
		t.Fatalf("slideshow: %+v", items[3].Slideshow)
	}
	if b, ok := s2.Blob(items[3].Slideshow.Slides[0].Temp + ":full"); !ok || string(b) != "full" {
		t.Fatal("slide blob missing")
	}
	if items[4].Media == nil || items[4].Media.Path != "/x/clip.mp4" {
		t.Fatalf("media: %+v", items[4].Media)
	}
}
