package qoi

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"image"
	"image/draw"
	"io"
)

type Header struct {
	Magic      [4]byte
	Width      uint32
	Height     uint32
	Channels   byte
	Colorspace byte
}

type Color struct {
	R, G, B, A uint8
}

type ColorDiff struct {
	R, G, B, A int8
}

const (
	TAG_OP_RUN   uint8 = 0b11000000
	TAG_OP_INDEX uint8 = 0b00000000
	TAG_OP_DIFF  uint8 = 0b01000000
	TAG_OP_LUMA  uint8 = 0b10000000
	TAG_OP_RGB   uint8 = 0b11111110
	TAG_OP_RGBA  uint8 = 0b11111111
)

var (
	defChannels   uint8 = 4
	defColorspace uint8 = 0
	magic               = []byte("qoif")
	endMarker           = []byte{0, 0, 0, 0, 0, 0, 0, 1}
)

func Encode(w io.Writer, img image.Image) error {
	buf := bufio.NewWriter(w)

	// Converting an Image to NRGBA
	// https://go.dev/blog/image-draw#TOC_6.
	b := img.Bounds()
	m := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(m, m.Bounds(), img, b.Min, draw.Src)

	width := m.Bounds().Dx()
	height := m.Bounds().Dy()
	pixels := m.Pix
	prev := Color{A: 255}
	// store seen before pixel
	cache := [64]Color{}
	lenRun := uint8(0)

	// write header
	buf.Write(magic)
	binary.Write(buf, binary.BigEndian, uint32(width))
	binary.Write(buf, binary.BigEndian, uint32(height))
	buf.WriteByte(defChannels)
	buf.WriteByte(defColorspace)

	lastPixel := width*height*int(defChannels) - int(defChannels)
	for offset := 0; offset <= lastPixel; offset += int(defChannels) {
		pix := Color{
			R: pixels[offset+0],
			G: pixels[offset+1],
			B: pixels[offset+2],
			A: pixels[offset+3],
		}
		// Record run of same pixel.
		// lenRun of 1..62 stored with a bias of -1
		if pix == prev {
			lenRun++
			if lenRun == 62 || offset == lastPixel {
				buf.WriteByte(TAG_OP_RUN | lenRun - 1)
				lenRun = 0
			}
			continue
		}
		if lenRun > 0 {
			buf.WriteByte(TAG_OP_RUN | lenRun - 1)
			lenRun = 0
		}

		// Encode as index of seen before pixel.
		// Store in 6-bit(0..63) with 0b00 tag.
		i := IndexHash(pix)
		if pix == cache[i] {
			buf.WriteByte(i)
			prev = pix
			continue
		}
		cache[i] = pix

		if pix.A == prev.A {
			diff := Diff(pix, prev)
			dr_dg := diff.R - diff.G
			db_dg := diff.B - diff.G

			// Encode as difference from previous pixel.
			// R, G, B diff must -2..1, A must be the same from previous.
			// R, G, B diffs are stored in 2-bit with -2 bias.
			if (diff.R >= -2 && diff.R <= 1) && (diff.G >= -2 && diff.G <= 1) && (diff.B >= -2 && diff.B <= 1) {
				diffByte := TAG_OP_DIFF | uint8(diff.R+2)<<4 | uint8(diff.G+2)<<2 | uint8(diff.B+2)
				buf.WriteByte(diffByte)
			} else if (diff.G >= -32 && diff.G <= 31) && (dr_dg >= -8 && dr_dg <= 7) && (db_dg >= -8 && db_dg <= 7) {
				buf.WriteByte(TAG_OP_LUMA | uint8(diff.G+32))
				buf.WriteByte(uint8(dr_dg+8)<<4 | uint8(db_dg+8))
			} else {
				buf.WriteByte(TAG_OP_RGB)
				buf.WriteByte(pix.R)
				buf.WriteByte(pix.G)
				buf.WriteByte(pix.B)
			}
			prev = pix
			continue
		}
		buf.WriteByte(TAG_OP_RGBA)
		buf.WriteByte(pix.R)
		buf.WriteByte(pix.G)
		buf.WriteByte(pix.B)
		buf.WriteByte(pix.A)
		prev = pix
	}
	buf.Write(endMarker)
	return buf.Flush()
}

func IndexHash(c Color) uint8 {
	return uint8((int(c.R)*3 + int(c.G)*5 + int(c.B)*7 + int(c.A)*11) % 64)
}

func Diff(c, prev Color) ColorDiff {
	return ColorDiff{
		R: int8(c.R) - int8(prev.R),
		G: int8(c.G) - int8(prev.G),
		B: int8(c.B) - int8(prev.B),
	}
}

func Decode(r io.Reader) (image.Image, error) {
	hdr, err := decodeHeader(r)
	if err != nil {
		return nil, err
	}
	img := image.NewNRGBA(image.Rect(0, 0, int(hdr.Width), int(hdr.Height)))
	buf := bufio.NewReader(r)
	pixels := bytes.NewBuffer(img.Pix)

	pix := make([]byte, int(hdr.Channels))
	for {
		byte, err := buf.ReadByte()
		if err != nil {
			return img, err
		}
		switch byte {
		// Complete RGBA pixel
		case TAG_OP_RGBA:
			_, err = buf.Read(pix)
			if err != nil {
				return img, err
			}
			_, err = pixels.Write(pix)
			if err != nil {
				return img, err
			}
		// Alpha is similar to previous pixel
		case TAG_OP_RGB:
			_, err = buf.Read(pix[:3])
			if err != nil {
				return img, err
			}
			_, err = pixels.Write(pix)
			if err != nil {
				return img, err
			}
		// n run of previous pixel.
		case byte > TAG_OP_RUN:
			n := int(TAG_OP_RUN^byte) + 1
			for i := 0; i < n; i++ {
				_, err = pixels.Write(pix)
				if err != nil {
					return img, err
				}
			}
		case byte > TAG_OP_LUMA:

		}

	}

	return img, err
}

func decodeHeader(r io.Reader) (Header, error) {
	hdr := Header{}
	_, err := r.Read(hdr.Magic[:])
	if err != nil {
		return hdr, err
	}
	err = binary.Read(r, binary.BigEndian, &hdr.Width)
	if err != nil {
		return hdr, err
	}
	err = binary.Read(r, binary.BigEndian, &hdr.Height)
	if err != nil {
		return hdr, err
	}
	buf := bufio.NewReader(r)
	hdr.Channels, err = buf.ReadByte()
	if err != nil {
		return hdr, err
	}
	hdr.Colorspace, err = buf.ReadByte()
	if err != nil {
		return hdr, err
	}
	return hdr, nil
}
