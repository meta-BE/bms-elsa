package gateway_test

import (
	"os"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
)

func TestParseLR2IR_Full(t *testing.T) {
	body, err := os.ReadFile("testdata/registered_full.html")
	if err != nil {
		t.Fatal(err)
	}

	resp, err := gateway.ParseLR2IRResponse(string(body))
	if err != nil {
		t.Fatal(err)
	}

	if !resp.Registered {
		t.Fatal("Registered should be true")
	}
	if resp.Genre != "DEMENTIA PROVECTO" {
		t.Errorf("Genre = %q, want %q", resp.Genre, "DEMENTIA PROVECTO")
	}
	if resp.Title != "Anima Mundi -Incipiens Finis- [EX]" {
		t.Errorf("Title = %q, want %q", resp.Title, "Anima Mundi -Incipiens Finis- [EX]")
	}
	if resp.Artist != "Limstella BGA:アカツキユウ / obj:仙人掌の人" {
		t.Errorf("Artist = %q, want to contain %q", resp.Artist, "Limstella")
	}
	if len(resp.Tags) != 2 || resp.Tags[0] != "Stella" || resp.Tags[1] != "st2" {
		t.Errorf("Tags = %v, want [Stella st2]", resp.Tags)
	}
	if resp.BodyURL != "http://manbow.nothing.sh/event/event.cgi?action=More_def&num=78&event=104" {
		t.Errorf("BodyURL = %q, want URL containing manbow.nothing.sh", resp.BodyURL)
	}
	if resp.DiffURL != "http://absolute.pv.land.to/uploader/src/up5912.zip" {
		t.Errorf("DiffURL = %q, want URL containing up5912.zip", resp.DiffURL)
	}
	if resp.Notes != "汎用up5912" {
		t.Errorf("Notes = %q, want %q", resp.Notes, "汎用up5912")
	}
	if resp.BPM != "95 - 190" {
		t.Errorf("BPM = %q, want %q", resp.BPM, "95 - 190")
	}
	if resp.Level != "☆21" {
		t.Errorf("Level = %q, want %q", resp.Level, "☆21")
	}
	if resp.Keys != "7KEYS" {
		t.Errorf("Keys = %q, want %q", resp.Keys, "7KEYS")
	}
	if resp.JudgeRank != "EASY" {
		t.Errorf("JudgeRank = %q, want %q", resp.JudgeRank, "EASY")
	}
}

func TestParseLR2IR_Minimal(t *testing.T) {
	body, err := os.ReadFile("testdata/registered_minimal.html")
	if err != nil {
		t.Fatal(err)
	}

	resp, err := gateway.ParseLR2IRResponse(string(body))
	if err != nil {
		t.Fatal(err)
	}

	if !resp.Registered {
		t.Fatal("Registered should be true")
	}
	if resp.Genre != "ELECTRO" {
		t.Errorf("Genre = %q, want %q", resp.Genre, "ELECTRO")
	}
	if resp.Title != "Lovers," {
		t.Errorf("Title = %q, want %q", resp.Title, "Lovers,")
	}
	if resp.Artist != "SHANG0" {
		t.Errorf("Artist = %q, want %q", resp.Artist, "SHANG0")
	}
	if len(resp.Tags) != 0 {
		t.Errorf("Tags = %v, want empty", resp.Tags)
	}
	if resp.BodyURL != "" {
		t.Errorf("BodyURL = %q, want empty", resp.BodyURL)
	}
	if resp.DiffURL != "" {
		t.Errorf("DiffURL = %q, want empty", resp.DiffURL)
	}
	if resp.Notes != "" {
		t.Errorf("Notes = %q, want empty", resp.Notes)
	}
}

func TestParseLR2IR_Unregistered(t *testing.T) {
	body, err := os.ReadFile("testdata/unregistered.html")
	if err != nil {
		t.Fatal(err)
	}

	resp, err := gateway.ParseLR2IRResponse(string(body))
	if err != nil {
		t.Fatal(err)
	}

	if resp.Registered {
		t.Fatal("Registered should be false")
	}
}
