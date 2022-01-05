package qoi

import (
	"bytes"
	"image/png"
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

}
