package modules

import (
	"os"
	"path/filepath"
	"testing"

	"wordprojector/internal/bible"
	"wordprojector/internal/db"
	"wordprojector/internal/songs"
)

const samples = "/private/tmp/claude-502/-Users-denis--claude-spares-spare-Bwaa8m/fcc8f251-416d-4bfc-8f97-931db0c2c74e/scratchpad/samples"

func TestSongbookSQLiteRoundTrip(t *testing.T) {
	src := filepath.Join(samples, "englishworship.sps")
	if _, err := os.Stat(src); err != nil {
		t.Skip("sample not present")
	}
	d, err := db.Open(filepath.Join(t.TempDir(), "spData.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	im := &Importer{DB: d}
	id, err := im.ImportSongbook(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	repo := &songs.Repo{DB: d}
	list, err := repo.SongsOfBook(id)
	if err != nil || len(list) != 37 {
		t.Fatalf("songs: %d %v", len(list), err)
	}
	if list[0].Title != "Above all" || list[0].Font.PointSize != 28 || list[0].Color != "#ffffff" || list[0].Count != 3 {
		t.Fatalf("%+v", list[0])
	}
	out := filepath.Join(t.TempDir(), "out.sps")
	if _, err := im.ExportSongbook(out, id, nil); err != nil {
		t.Fatal(err)
	}
	id2, err := im.ImportSongbook(out, nil)
	if err != nil {
		t.Fatal(err)
	}
	list2, _ := repo.SongsOfBook(id2)
	if len(list2) != 37 || list2[5].Text != list[5].Text {
		t.Fatal("re-import mismatch")
	}
	// legacy text format
	legacy := filepath.Join(t.TempDir(), "old.sps")
	os.WriteFile(legacy, []byte("##1.07\n#Old Book\n#Line1@%Line2\n1#$#First#$#0#$#C#$#W#$#M#$#Verse 1@%la la@$Refrain@%ref#$#Arial,28,-1,5,50,0,0,0,0,0#$##$##$#note\n2#$#Second#$#0#$##$##$##$#Verse 1@%b\n"), 0o644)
	id3, err := im.ImportSongbook(legacy, nil)
	if err != nil {
		t.Fatal(err)
	}
	l3, _ := repo.SongsOfBook(id3)
	if len(l3) != 2 || l3[0].Text != "Verse 1\nla la\n\nRefrain\nref" || l3[0].Notes != "note" {
		t.Fatalf("legacy: %+v", l3)
	}
	sb, _ := repo.Songbook(id3)
	if sb.Info != "Line1\nLine2" {
		t.Fatalf("info %q", sb.Info)
	}
	// xml format
	x := filepath.Join(t.TempDir(), "x.sps")
	os.WriteFile(x, []byte(`<?xml version="1.0" encoding="UTF-8"?><spSongBook version="2.0"><SongBook><title>X</title><info>i</info></SongBook><Song number="7"><title>T</title><category>3</category><tune>C</tune><words>w</words><music>m</music><song_text>Verse 1
abc</song_text><notes>n</notes><use_private>false</use_private><alignment>1,2</alignment><color>4294967295</color><font>Arial,28,-1,5,50,0,0,0,0,0</font><background></background><count>0</count><date></date></Song></spSongBook>`), 0o644)
	id4, err := im.ImportSongbook(x, nil)
	if err != nil {
		t.Fatal(err)
	}
	l4, _ := repo.SongsOfBook(id4)
	if len(l4) != 1 || l4[0].Number != 7 || l4[0].AlignH != 2 || l4[0].Text != "Verse 1\nabc" {
		t.Fatalf("xml: %+v", l4)
	}
}

func TestBibleRoundTrip(t *testing.T) {
	src := filepath.Join(samples, "sp_kjv.spb")
	if _, err := os.Stat(src); err != nil {
		t.Skip("sample not present")
	}
	d, err := db.Open(filepath.Join(t.TempDir(), "spData.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	im := &Importer{DB: d}
	id, err := im.ImportBible(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	br := bible.NewRepo(d)
	books, _ := br.Books(id)
	if len(books) != 66 || books[0].Name != "Genesis" || books[0].Chapters != 50 {
		t.Fatalf("books %d %+v", len(books), books[0])
	}
	ch, _ := br.Chapter(id, 1, 1)
	if len(ch) != 31 || ch[0].IDs != "B001C001V001" {
		t.Fatalf("chapter %d", len(ch))
	}
	p, _ := br.VerseAndCaption(id, "B001C001V001", true)
	if p.Caption != "Genesis 1:1 (KJV)" || p.Text == "" {
		t.Fatalf("%+v", p)
	}
	p2, _ := br.VerseAndCaption(id, "B001C001V001,B001C001V002", false)
	if p2.Caption != "Genesis 1:1-2" || p2.Text[:4] != "(1) " {
		t.Fatalf("%+v", p2)
	}
	hits, _, err := br.Search(id, "in the beginning", true, false, 0, 0)
	if err != nil || len(hits) == 0 || hits[0].Book != "Genesis" {
		t.Fatalf("search %d %v", len(hits), err)
	}
	out := filepath.Join(t.TempDir(), "out.spb")
	if _, err := im.ExportBible(out, id, nil); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(src)
	b, _ := os.ReadFile(out)
	if len(a) != len(b) {
		t.Fatalf("export size differs: %d vs %d", len(a), len(b))
	}
}
