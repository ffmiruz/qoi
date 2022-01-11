package qoi

import (
	"bytes"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncode(t *testing.T) {
	files, err := filepath.Glob("qoi_test_images/*.png")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range files {
		f, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		m, err := png.Decode(f)
		if err != nil {
			t.Fatal(err)
		}
		buf := bytes.NewBuffer([]byte{})
		err = Encode(buf, m)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(strings.Replace(p, ".png", ".qoi", 1))
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(data, buf.Bytes()) {
			t.Errorf("%s: wrongly encoded", p)
		}
	}

}

func TestDecode(t *testing.T) {
	file, err := os.Open("qoi_test_images/testcard_rgba.qoi")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	img, err := Decode(file)
	if err != nil {
		t.Error(err)
	}
	b := new(bytes.Buffer)
	err = png.Encode(b, img)
	if err != nil {
		t.Fatal(err)
	}
	fpng, err := os.Open("qoi_test_images/testcard_rgba.png")
	if err != nil {
		t.Fatal(err)
	}
	defer fpng.Close()
	std, err := io.ReadAll(fpng)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Bytes()) != len(std) {
		t.Errorf("expect len of %v instead of %v", len(std), len(b.Bytes()))
	}
	if !bytes.Equal(b.Bytes(), std) {
		t.Errorf("wrongly decoded")
	}
}
