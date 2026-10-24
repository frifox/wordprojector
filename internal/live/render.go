// Package live holds the projection state machine: what is being shown, the
// list in the live panel, and the fully resolved slide description each
// display window renders.
package live

import (
	"wordprojector/internal/qt"
)

// Background describes what is painted behind the text.
type Background struct {
	Type  string `json:"type"` // "color" | "image"
	Color string `json:"color,omitempty"`
	URL   string `json:"url,omitempty"`
	Fit   string `json:"fit,omitempty"` // how an image fills the screen: "cover", "contain" or "fill" (stretch)
	// Clear is how see-through a color is, in percent (0 is solid). Under is what shows through it: the picture of
	// the theme, else black.
	Clear int         `json:"clear,omitempty"`
	Under *Background `json:"under,omitempty"`
}

// TextStyle is the resolved style for a block of text.
type TextStyle struct {
	Font   qt.Font `json:"font"`
	Color  string  `json:"color"`
	Shadow bool    `json:"shadow"`
	Blur   bool    `json:"blur"`
	AlignV int     `json:"alignV"`
	AlignH int     `json:"alignH"`
	ScaleH  int    `json:"scaleH"`
}

// Passage is one translation's text and caption on a bible slide.
type Passage struct {
	Text    string `json:"text"`
	Caption string `json:"caption"`
}

// BibleSlide renders one or more translations of a verse range.
type BibleSlide struct {
	Parts           []Passage `json:"parts"`
	Text            TextStyle `json:"text"`
	CaptionFont     qt.Font   `json:"captionFont"`
	CaptionColor    string    `json:"captionColor"`
	CaptionAlign    int       `json:"captionAlign"`
	CaptionPosition int       `json:"captionPosition"`
	CaptionScaleH   int       `json:"captionScaleH"`
	ScreenUse       int       `json:"screenUse"`
	ScreenPosition  int       `json:"screenPosition"`
	AddBK           bool      `json:"addBk"`
	RecBK           string    `json:"recBk"`
	BkPadding       int       `json:"bkPadding"`
	BkOpacity       int       `json:"bkOpacity"`
}

// SongSlide renders one stanza.
type SongSlide struct {
	Text           string    `json:"text"`
	StanzaTitle    string    `json:"stanzaTitle"`
	Number         int       `json:"number"`
	Tune           string    `json:"tune"`
	IsLast         bool      `json:"isLast"`
	Style          TextStyle `json:"style"`
	ShowTitle      bool      `json:"showTitle"`
	ShowKey        bool      `json:"showKey"`
	ShowNumber     bool      `json:"showNumber"`
	InfoFont       qt.Font   `json:"infoFont"`
	InfoColor      string    `json:"infoColor"`
	InfoAlign      int       `json:"infoAlign"`
	InfoScaleH     int       `json:"infoScaleH"`
	ShowEnding     bool      `json:"showEnding"`
	EndingFont     qt.Font   `json:"endingFont"`
	EndingColor    string    `json:"endingColor"`
	EndingType     int       `json:"endingType"`
	EndingPosition int       `json:"endingPosition"`
	EndingScaleH   int       `json:"endingScaleH"`
	ScreenUse      int       `json:"screenUse"`
	ScreenPosition int       `json:"screenPosition"`
	AddBK          bool      `json:"addBk"`
	RecBK          string    `json:"recBk"`
	BkPadding      int       `json:"bkPadding"`
	BkOpacity      int       `json:"bkOpacity"`
	// Stanzas is the text of every stanza of the song: all of them are fitted to the screen together, so
	// the text has the same size on every stanza.
	Stanzas        []string  `json:"stanzas"`
	WordsBy        string    `json:"wordsBy"`
	MusicBy        string    `json:"musicBy"`
}

// AnnounceSlide renders one announcement slide.
type AnnounceSlide struct {
	Text  string    `json:"text"`
	Style TextStyle `json:"style"`
}

// PictureSlide renders one picture.
type PictureSlide struct {
	URL         string `json:"url"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	ExpandSmall bool   `json:"expandSmall"`
	FitType     int    `json:"fitType"`
}

// VideoSlide plays a media file.
type VideoSlide struct {
	URL   string `json:"url"`
	Name  string `json:"name"`
	Muted bool   `json:"muted"`
}

// Slide is what one display window renders right now.
type Slide struct {
	Kind       string         `json:"kind"` // passive | clear | bible | song | announce | picture | video | test
	Fade       bool           `json:"fade"`
	FadeMillis int            `json:"fadeMillis,omitempty"`
	Background Background     `json:"background"`
	Bible      *BibleSlide    `json:"bible,omitempty"`
	Song       *SongSlide     `json:"song,omitempty"`
	Announce   *AnnounceSlide `json:"announce,omitempty"`
	Picture    *PictureSlide  `json:"picture,omitempty"`
	Video      *VideoSlide    `json:"video,omitempty"`
	Test       *TestSlide     `json:"test,omitempty"`
}

// TestSlide is the pattern shown while the display screens are being tested.
type TestSlide struct {
	Disp   int `json:"disp"`
	Screen int `json:"screen"`
	Millis int `json:"millis"` // how long the test lasts, for the countdown
	Width  int `json:"width"`  // pixel size of the screen (0 when unknown)
	Height int `json:"height"`
}

// DisplayState pairs a display number (1-4) with its slide.
type DisplayState struct {
	Disp   int   `json:"disp"`
	Screen int   `json:"screen"`
	// Width and Height are the pixel size of the screen (0 when unknown, e.g. in browser mode); the operator
	// preview uses their ratio so it shows the same layout as that screen.
	Width  int   `json:"width"`
	Height int   `json:"height"`
	Slide  Slide `json:"slide"`
	// Preview is what Slide would be if the presentation were running (equal to Slide while it is).
	Preview Slide `json:"preview"`
}

// Item is one row of the live list.
type Item struct {
	Text  string `json:"text"`
	Image string `json:"image,omitempty"`
}

// VideoState is reported by the display window playing the video.
type VideoState struct {
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
	Playing  bool    `json:"playing"`
	Ended    bool    `json:"ended"`
	Volume   int     `json:"volume"`
	Muted    bool    `json:"muted"`
}

// State is broadcast to every connected client.
type State struct {
	Version      int64          `json:"version"`
	Showing      bool           `json:"showing"`
	Cleared      bool           `json:"cleared"`
	DisplayOn    bool           `json:"displayOn"`
	SingleScreen bool           `json:"singleScreen"`
	Type         string         `json:"type"`
	Title        string         `json:"title"`
	Notes        string         `json:"notes"`
	Items        []Item         `json:"items"`
	Index        int            `json:"index"`
	Selection    []int          `json:"selection"`
	MultiVerse   bool           `json:"multiVerse"`
	Displays     []DisplayState `json:"displays"`
	Video        VideoState     `json:"video"`
	Controls     Controls       `json:"controls"`
	// Test is true while the display test pattern is shown on the display screens.
	Test bool `json:"test"`
}

// Controls configures the on-screen buttons shown in single screen mode.
type Controls struct {
	Visible    bool    `json:"visible"`
	ButtonSize int     `json:"buttonSize"`
	AlignV     int     `json:"alignV"`
	AlignH     int     `json:"alignH"`
	Opacity    float64 `json:"opacity"`
}
