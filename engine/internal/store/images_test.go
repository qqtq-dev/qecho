package store

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

const tinyPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func TestSaveImageDedupesByContent(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	data, err := base64.StdEncoding.DecodeString(tinyPNG)
	if err != nil {
		t.Fatal(err)
	}

	first, err := st.SaveImage(data, "tree.png")
	if err != nil {
		t.Fatal(err)
	}
	if first.Deduped {
		t.Fatal("first upload should not be deduped")
	}
	if len(first.Filename) != 64+4 || !strings.HasSuffix(first.Filename, ".png") {
		t.Fatalf("unexpected filename %q", first.Filename)
	}
	if first.Mime != "image/png" || first.Size != int64(len(data)) {
		t.Fatalf("unexpected meta %+v", first)
	}
	path, err := st.ImagePath(first.Filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("image file missing: %v", err)
	}
	if _, err := st.ImagePath("../quiz.db"); err == nil {
		t.Fatal("expected traversal to be rejected")
	}

	second, err := st.SaveImage(data, "another-name.png")
	if err != nil {
		t.Fatal(err)
	}
	if !second.Deduped || second.Filename != first.Filename {
		t.Fatalf("expected dedupe, got %+v", second)
	}
}

func TestSaveImageRejectsNonImage(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.SaveImage([]byte("not an image"), "x.txt"); err == nil {
		t.Fatal("expected unsupported type error")
	}
}

func TestNormalizeImageRef(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "", false},
		{"abc.png", "abc.png", false},
		{"https://example.com/a.png", "https://example.com/a.png", false},
		{"../quiz.db", "", true},
		{"images/a.png", "", true},
	}
	for _, c := range cases {
		got, err := NormalizeImageRef(c.in)
		if (err != nil) != c.wantErr || got != c.want {
			t.Fatalf("NormalizeImageRef(%q) = %q, %v", c.in, got, err)
		}
	}
}

func TestOptionImageRoundTrip(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	bank, err := st.CreateBank("图片题", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	q, err := st.CreateQuestion(bank.ID, QuestionInput{
		Type:      "single",
		Stem:      "看图选择",
		StemImage: "a1b2.png",
		Answer:    []string{"A"},
		Options: []QuestionOption{
			{Key: "A", Content: "甲", Image: "c3d4.png"},
			{Key: "B", Content: "乙"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if q.StemImage != "a1b2.png" || q.Options[0].Image != "c3d4.png" || q.Options[1].Image != "" {
		t.Fatalf("images not persisted: %+v", q)
	}
	got, err := st.GetQuestion(q.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Options[0].Image != "c3d4.png" {
		t.Fatalf("option image not loaded: %+v", got.Options[0])
	}
}
