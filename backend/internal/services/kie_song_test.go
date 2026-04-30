package services

import (
	"context"
	"strings"
	"testing"
)

func TestPreprocessMashupAudioBypassesNonMP3(t *testing.T) {
	g := NewKieSongGenerator("test", nil, "missing-ffmpeg")

	in := []byte{1, 2, 3}
	out, name, err := g.preprocessMashupAudio(context.Background(), "track.wav", in)
	if err != nil {
		t.Fatalf("preprocessMashupAudio returned error: %v", err)
	}
	if name != "track.wav" {
		t.Fatalf("unexpected filename: %s", name)
	}
	if string(out) != string(in) {
		t.Fatalf("non-mp3 data was modified")
	}
}

func TestPreprocessMashupAudioMissingFFmpeg(t *testing.T) {
	g := NewKieSongGenerator("test", nil, "definitely-missing-ffmpeg")

	_, _, err := g.preprocessMashupAudio(context.Background(), "track.mp3", []byte("fake mp3"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ffmpeg not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}
