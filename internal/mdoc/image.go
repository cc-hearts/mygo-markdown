package mdoc

import (
	"encoding/base64"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

func isRemoteImage(src string) bool {
	return strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")
}

type remoteEntry struct {
	mu      sync.Mutex
	bmp     *ui.Bitmap
	loading bool
	failed  bool
}

var remoteImages sync.Map

// remoteBitmap returns a fetched remote image, or nil while it loads. The
// first caller starts the fetch; OnAsync repaints the view when it lands.
func remoteBitmap(src string, done func()) *ui.Bitmap {
	e := &remoteEntry{loading: true}
	actual, loaded := remoteImages.LoadOrStore(src, e)
	if loaded {
		e = actual.(*remoteEntry)
		e.mu.Lock()
		bmp := e.bmp
		e.mu.Unlock()
		return bmp
	}
	go func() {
		fail := func() {
			e.mu.Lock()
			e.loading, e.failed = false, true
			e.mu.Unlock()
			if done != nil {
				done()
			}
		}
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Get(src)
		if err != nil {
			fail()
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			fail()
			return
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
		if err != nil {
			fail()
			return
		}
		bmp, err := ui.DecodeBitmap(data)
		e.mu.Lock()
		e.loading = false
		if err != nil {
			e.failed = true
		} else {
			e.bmp = bmp
		}
		e.mu.Unlock()
		if done != nil {
			done()
		}
	}()
	return nil
}

func resolveImagePath(src, base string) string {
	src = strings.TrimSpace(src)
	if src == "" || strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") ||
		strings.HasPrefix(src, "data:") {
		return ""
	}
	src = strings.TrimPrefix(src, "file://")
	if unescaped, err := url.PathUnescape(src); err == nil {
		src = unescaped
	}
	if filepath.IsAbs(src) {
		return src
	}
	if base == "" {
		return src
	}
	return filepath.Join(base, src)
}

type imageEntry struct {
	mod  time.Time
	size int64
	bmp  *ui.Bitmap
}

var imageCache sync.Map

func loadBitmap(path string) *ui.Bitmap {
	st, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if v, ok := imageCache.Load(path); ok {
		e := v.(imageEntry)
		if e.mod.Equal(st.ModTime()) && e.size == st.Size() {
			return e.bmp
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	bmp, err := ui.DecodeBitmap(data)
	if err != nil {
		return nil
	}
	imageCache.Store(path, imageEntry{mod: st.ModTime(), size: st.Size(), bmp: bmp})
	return bmp
}

// HeadingSize is the native heading scale, shared with the block editor.
func HeadingSize(level int, body float32) float32 {
	switch level {
	case 1:
		return body * 2.1
	case 2:
		return body * 1.65
	case 3:
		return body * 1.4
	case 4:
		return body * 1.2
	case 5:
		return body
	default:
		return body * 0.875
	}
}

func htmlAttr(tag, name string) string {
	low := strings.ToLower(tag)
	i := strings.Index(low, name+"=")
	if i < 0 {
		return ""
	}
	rest := tag[i+len(name)+1:]
	if rest == "" {
		return ""
	}
	q := rest[0]
	if q != '"' && q != '\'' {
		end := strings.IndexAny(rest, " \t>")
		if end < 0 {
			return rest
		}
		return rest[:end]
	}
	end := strings.IndexByte(rest[1:], q)
	if end < 0 {
		return ""
	}
	return rest[1 : end+1]
}

// LoadImage returns the bitmap of an image a document links to: a file
// relative to baseDir, a data: URL, or a remote image, fetched once in the
// background (nil until then; onLoad runs on another goroutine when it
// lands). It returns nil for what cannot be read.
func LoadImage(src, baseDir string, onLoad func()) *ui.Bitmap {
	src = strings.TrimSpace(src)
	switch {
	case src == "":
		return nil
	case isRemoteImage(src):
		return remoteBitmap(src, onLoad)
	case strings.HasPrefix(src, "data:"):
		return dataBitmap(src)
	}
	path := resolveImagePath(src, baseDir)
	if path == "" {
		return nil
	}
	return loadBitmap(path)
}

var dataImages sync.Map

// dataBitmap decodes a base64 data: URL, as pasted images are kept.
func dataBitmap(src string) *ui.Bitmap {
	if v, ok := dataImages.Load(src); ok {
		bmp, _ := v.(*ui.Bitmap)
		return bmp
	}
	var bmp *ui.Bitmap
	if comma := strings.IndexByte(src, ','); comma > 0 && strings.Contains(src[:comma], ";base64") {
		if data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(src[comma+1:])); err == nil {
			bmp, _ = ui.DecodeBitmap(data)
		}
	}
	dataImages.Store(src, bmp)
	return bmp
}

// HTMLOnlyImage reports the image of an HTML block that holds nothing but
// one <img>, as READMEs center their logos, with its width and height.
func HTMLOnlyImage(html string) (src, alt, title string, w, h float32, ok bool) {
	return htmlOnlyImage(html)
}

// htmlOnlyImage recognises an HTML block that holds nothing but one image, as
// READMEs often do with <p align="center"><img ...></p>, and turns it into a
// native image block. Text around the image makes it a normal HTML block.
func htmlOnlyImage(s string) (src, alt, title string, w, h float32, ok bool) {
	low := strings.ToLower(s)
	start := strings.Index(low, "<img")
	if start < 0 {
		return "", "", "", 0, 0, false
	}
	end := strings.Index(s[start:], ">")
	if end < 0 {
		return "", "", "", 0, 0, false
	}
	tag := s[start : start+end]
	src = htmlAttr(tag, "src")
	if src == "" {
		return "", "", "", 0, 0, false
	}
	w = cssSize(htmlAttr(tag, "width"))
	h = cssSize(htmlAttr(tag, "height"))
	// Everything outside the tag must be whitespace or other tags.
	rest := s[:start] + s[start+end+1:]
	for i := 0; i < len(rest); {
		if rest[i] == '<' {
			j := strings.IndexByte(rest[i:], '>')
			if j < 0 {
				return "", "", "", 0, 0, false
			}
			i += j + 1
			continue
		}
		if !strings.ContainsRune(" \t\r\n", rune(rest[i])) {
			return "", "", "", 0, 0, false
		}
		i++
	}
	return src, htmlAttr(tag, "alt"), htmlAttr(tag, "title"), w, h, true
}

// cssSize reads an HTML width/height such as 128, "128px" or "12.5em".
func cssSize(v string) float32 {
	v = strings.TrimSpace(strings.TrimSuffix(strings.ToLower(v), "px"))
	if v == "" {
		return 0
	}
	n, err := strconv.ParseFloat(v, 32)
	if err != nil || n < 0 {
		return 0
	}
	return float32(n)
}
