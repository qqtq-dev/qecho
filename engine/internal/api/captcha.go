package api

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math/big"
	"net/http"
	"strings"
	"time"

	"quizbox/engine/internal/authx"
	"quizbox/engine/internal/response"
)

const (
	captchaCharset = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	captchaLength  = 4
	captchaCell    = 5
	captchaScale   = 5
	captchaGap     = 9
)

// 5x7 bitmap glyphs, one byte per row, bit 4 = leftmost pixel.
var captchaGlyphs = map[byte][7]byte{
	'A': {0b01110, 0b10001, 0b10001, 0b11111, 0b10001, 0b10001, 0b10001},
	'B': {0b11110, 0b10001, 0b10001, 0b11110, 0b10001, 0b10001, 0b11110},
	'C': {0b01111, 0b10000, 0b10000, 0b10000, 0b10000, 0b10000, 0b01111},
	'D': {0b11110, 0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b11110},
	'E': {0b11111, 0b10000, 0b10000, 0b11110, 0b10000, 0b10000, 0b11111},
	'F': {0b11111, 0b10000, 0b10000, 0b11110, 0b10000, 0b10000, 0b10000},
	'G': {0b01111, 0b10000, 0b10000, 0b10111, 0b10001, 0b10001, 0b01110},
	'H': {0b10001, 0b10001, 0b10001, 0b11111, 0b10001, 0b10001, 0b10001},
	'J': {0b00111, 0b00010, 0b00010, 0b00010, 0b00010, 0b10010, 0b01100},
	'K': {0b10001, 0b10010, 0b10100, 0b11000, 0b10100, 0b10010, 0b10001},
	'M': {0b10001, 0b11011, 0b10101, 0b10101, 0b10001, 0b10001, 0b10001},
	'N': {0b10001, 0b11001, 0b10101, 0b10011, 0b10001, 0b10001, 0b10001},
	'P': {0b11110, 0b10001, 0b10001, 0b11110, 0b10000, 0b10000, 0b10000},
	'Q': {0b01110, 0b10001, 0b10001, 0b10001, 0b10101, 0b10010, 0b01101},
	'R': {0b11110, 0b10001, 0b10001, 0b11110, 0b10100, 0b10010, 0b10001},
	'S': {0b01111, 0b10000, 0b10000, 0b01110, 0b00001, 0b00001, 0b11110},
	'T': {0b11111, 0b00100, 0b00100, 0b00100, 0b00100, 0b00100, 0b00100},
	'U': {0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b01110},
	'V': {0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b01010, 0b00100},
	'W': {0b10001, 0b10001, 0b10001, 0b10101, 0b10101, 0b11011, 0b10001},
	'X': {0b10001, 0b10001, 0b01010, 0b00100, 0b01010, 0b10001, 0b10001},
	'Y': {0b10001, 0b10001, 0b01010, 0b00100, 0b00100, 0b00100, 0b00100},
	'Z': {0b11111, 0b00001, 0b00010, 0b00100, 0b01000, 0b10000, 0b11111},
	'2': {0b01110, 0b10001, 0b00001, 0b00010, 0b00100, 0b01000, 0b11111},
	'3': {0b11111, 0b00010, 0b00100, 0b00010, 0b00001, 0b10001, 0b01110},
	'4': {0b00010, 0b00110, 0b01010, 0b10010, 0b11111, 0b00010, 0b00010},
	'5': {0b11111, 0b10000, 0b11110, 0b00001, 0b00001, 0b10001, 0b01110},
	'6': {0b00110, 0b01000, 0b10000, 0b11110, 0b10001, 0b10001, 0b01110},
	'7': {0b11111, 0b00001, 0b00010, 0b00100, 0b01000, 0b01000, 0b01000},
	'8': {0b01110, 0b10001, 0b10001, 0b01110, 0b10001, 0b10001, 0b01110},
	'9': {0b01110, 0b10001, 0b10001, 0b01111, 0b00001, 0b00010, 0b01100},
}

var captchaInk = []color.RGBA{
	{28, 92, 66, 255},
	{23, 61, 96, 255},
	{63, 74, 46, 255},
	{88, 52, 34, 255},
}

func randomInt(min, max int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	if err != nil {
		return min
	}
	return min + int(n.Int64())
}

func randomCode() string {
	b := make([]byte, captchaLength)
	for i := range b {
		b[i] = captchaCharset[randomInt(0, len(captchaCharset)-1)]
	}
	return string(b)
}

func renderCaptcha(code string) ([]byte, error) {
	width := captchaLength*captchaCell*captchaScale + (captchaLength-1)*captchaGap + 16
	height := 7*captchaScale + 16
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	bg := color.RGBA{246, 250, 248, 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, bg)
		}
	}

	for i := 0; i < captchaLength; i++ {
		ink := captchaInk[randomInt(0, len(captchaInk)-1)]
		glyph, ok := captchaGlyphs[code[i]]
		if !ok {
			continue
		}
		originX := 8 + i*(captchaCell*captchaScale+captchaGap)
		originY := 8 + randomInt(-3, 3)
		for row := 0; row < 7; row++ {
			for col := 0; col < 5; col++ {
				if glyph[row]&(1<<(4-col)) == 0 {
					continue
				}
				px := originX + col*captchaScale
				py := originY + row*captchaScale
				for dy := 0; dy < captchaScale; dy++ {
					for dx := 0; dx < captchaScale; dx++ {
						img.Set(px+dx, py+dy, ink)
					}
				}
			}
		}
	}

	noise := color.RGBA{196, 214, 205, 255}
	for i := 0; i < 3; i++ {
		x0, y0 := randomInt(0, width/3), randomInt(0, height)
		slope := randomInt(-1, 1)
		for x := x0; x < width && x < x0+width/2; x++ {
			y := y0 + (x-x0)*slope/2
			if y < 0 || y >= height {
				continue
			}
			img.Set(x, y, noise)
			if y+1 < height {
				img.Set(x, y+1, noise)
			}
		}
	}
	for i := 0; i < 120; i++ {
		img.Set(randomInt(0, width-1), randomInt(0, height-1), noise)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *Server) captcha(w http.ResponseWriter, r *http.Request) {
	code := randomCode()
	data, err := renderCaptcha(code)
	if err != nil {
		response.Internal(w, err)
		return
	}
	id, err := authx.NewToken()
	if err != nil {
		response.Internal(w, err)
		return
	}
	now := time.Now()
	s.captchaMu.Lock()
	for key, item := range s.captchas {
		if now.After(item.expires) {
			delete(s.captchas, key)
		}
	}
	s.captchas[id[:12]] = captchaEntry{code: code, expires: now.Add(5 * time.Minute)}
	s.captchaMu.Unlock()
	response.OK(w, map[string]string{
		"id":    id[:12],
		"image": "data:image/png;base64," + base64.StdEncoding.EncodeToString(data),
	})
}

func (s *Server) checkCaptcha(id, answer string) bool {
	answer = strings.ToUpper(strings.TrimSpace(answer))
	if id == "" || answer == "" {
		return false
	}
	s.captchaMu.Lock()
	item, ok := s.captchas[id]
	if ok {
		delete(s.captchas, id)
	}
	s.captchaMu.Unlock()
	if !ok || time.Now().After(item.expires) {
		return false
	}
	return answer == item.code
}
