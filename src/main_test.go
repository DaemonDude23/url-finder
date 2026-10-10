package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestIsURL(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  bool
	}{
		{"http://example.com", true}, {"https://example.com:443/a?q=b", true},
		{"http://127.0.0.1:8080", true}, {"https://[::1]/", true},
		{"invalid-string", false}, {"/relative/path", false}, {"ftp://example.com", false},
		{"https://", false}, {"http://:80", false}, {"http://bad host", false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			if got := isURL(tc.input); got != tc.want {
				t.Errorf("isURL(%q)=%v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestFilterResults(t *testing.T) {
	results := [][]string{
		{"URL", "source", "ok", "127.0.0.1", "200", ""},
		{"URL", "source", "dns", "", "", ""},
		{"URL", "source", "http-error", "127.0.0.1", "", ""},
		{"URL", "source", "missing", "127.0.0.1", "404", ""},
		{"URL", "source", "broken", "127.0.0.1", "500", ""},
		{"URL", "source", "redirect-ok", "127.0.0.1", "301 -> 200", ""},
		{"URL", "source", "redirect-stopped", "127.0.0.1", "301 -> 302", ""},
	}
	for _, tc := range []struct {
		name             string
		noQuery, showAll bool
		want             []string
	}{
		{"failures", false, false, []string{"dns", "http-error", "missing", "broken", "redirect-stopped"}},
		{"all", false, true, []string{"ok", "dns", "http-error", "missing", "broken", "redirect-ok", "redirect-stopped"}},
		{"no-query", true, false, []string{"ok", "dns", "http-error", "missing", "broken", "redirect-ok", "redirect-stopped"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, row := range filterResults(results, tc.noQuery, tc.showAll) {
				got = append(got, row[2])
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExtractURLs(t *testing.T) {
	input := `<a href='http://one.example/a'>one</a> https://two.example/b?q=1&x=2 /relative ftp://ignored.example "https://three.example/"`
	want := []string{"http://one.example/a", "https://two.example/b?q=1&x=2", "https://three.example/"}
	if got := extractURLs(input); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestProcessURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<a href='http://linked.example/page'>one</a> https://another.example/resource /relative`)
	}))
	defer server.Close()
	got := processURL(server.URL)
	want := [][]string{{"URL", server.URL, server.URL, "", "", "", "", ""}, {"URL", server.URL, "http://linked.example/page", "", "", "", "", ""}, {"URL", server.URL, "https://another.example/resource", "", "", "", "", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

type failingBody struct{ closed bool }

func (*failingBody) Read([]byte) (int, error) { return 0, errors.New("body failure") }
func (b *failingBody) Close() error           { b.closed = true; return nil }
func TestProcessURLFailures(t *testing.T) {
	body := &failingBody{}
	for _, tc := range []struct {
		name      string
		transport roundTripperFunc
	}{
		{"transport", func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") }},
		{"body", func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: body, Header: make(http.Header)}, nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := processURLWithClient("http://127.0.0.1", &http.Client{Transport: tc.transport}); len(got) != 0 {
				t.Errorf("unexpected results %v", got)
			}
		})
	}
	if !body.closed {
		t.Error("response body was not closed")
	}
}

func TestGetURLStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "/ok", http.StatusMovedPermanently)
		case "/chain":
			http.Redirect(w, r, "/redirect", http.StatusFound)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		case "/missing":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()
	for _, tc := range []struct{ path, status, redirect string }{{"/ok", "200", ""}, {"/missing", "404", ""}, {"/redirect", "301 -> 200", ""}, {"/chain", "302 -> 301", "/ok"}, {"/loop", "302 -> 302", "/loop"}} {
		t.Run(tc.path, func(t *testing.T) {
			ip, status, redirect := getURLStatus(server.URL + tc.path)
			if ip != "127.0.0.1" || status != tc.status || redirect != tc.redirect {
				t.Errorf("got (%q,%q,%q), want (127.0.0.1,%q,%q)", ip, status, redirect, tc.status, tc.redirect)
			}
		})
	}
	for _, raw := range []string{"%invalid", "ftp://127.0.0.1", "http://"} {
		if ip, status, redirect := getURLStatus(raw); ip != "" || status != "" || redirect != "" {
			t.Errorf("invalid URL returned %q %q %q", ip, status, redirect)
		}
	}
}

func TestHTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	client := &http.Client{Timeout: 20 * time.Millisecond}
	if got := processURLWithClient(server.URL, client); len(got) != 0 {
		t.Errorf("timed-out page returned %v", got)
	}
	if ip, status, _ := getURLStatusWithClient(server.URL, client); ip != "127.0.0.1" || status != "" {
		t.Errorf("timed-out probe returned %q %q", ip, status)
	}
}

func TestFilesAndDirectories(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"notes.MD": "first line\nhttps://one.example/a https://two.example/b\n", "nested/links.txt": "http://three.example/\n", "ignored.go": "https://ignored.example/\n", "long.txt": strings.Repeat("x", 70*1024) + " https://long.example/\n"}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if !isDirectory(root) || isFile(root) || !isFile(filepath.Join(root, "notes.MD")) || isDirectory(filepath.Join(root, "notes.MD")) || isFile(filepath.Join(root, "missing")) {
		t.Error("incorrect path classification")
	}
	rows := processFile(filepath.Join(root, "notes.MD"))
	if len(rows) != 2 || rows[0][7] != "2" || rows[1][7] != "2" || rows[0][1] != filepath.Join(root, "notes.MD") {
		t.Errorf("incorrect source/line metadata: %v", rows)
	}
	if got := processDirectory(root, " .md, .TXT "); len(got) != 4 {
		t.Errorf("got %d URLs, want 4: %v", len(got), got)
	}
	if got := processFile(filepath.Join(root, "missing")); len(got) != 0 {
		t.Errorf("missing file returned %v", got)
	}
	if got := processDirectory(filepath.Join(root, "missing"), ".txt"); len(got) != 0 {
		t.Errorf("missing directory returned %v", got)
	}
}

func TestAccessURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	for _, workers := range []int{-1, 0, 1, 4} {
		t.Run(strconv.Itoa(workers), func(t *testing.T) {
			rows := [][]string{{"File", "one", server.URL, "", "", "", "", "1"}, {"File", "two", server.URL, "", "", "", "", "2"}}
			got := accessURLs(rows, workers)
			if len(got) != 2 {
				t.Fatalf("got %d results, want 2", len(got))
			}
			sources := map[string]bool{}
			for _, row := range got {
				sources[row[1]] = true
				if row[3] != "127.0.0.1" || row[4] != "200" {
					t.Errorf("bad probe result %v", row)
				}
			}
			if !sources["one"] || !sources["two"] {
				t.Errorf("lost source metadata %v", got)
			}
		})
	}
	if got := accessURLs(nil, 0); got != nil {
		t.Errorf("empty input returned %v", got)
	}
}

func TestVersion(t *testing.T) {
	old := version
	defer func() { version = old }()
	version = ""
	if got := (CLIArgs{}).Version(); got != "url-finder "+strings.TrimSpace(sourceVersion) {
		t.Errorf("source version: %q", got)
	}
	version = "1.2.3"
	if got := (CLIArgs{}).Version(); got != "url-finder 1.2.3" {
		t.Errorf("release version: %q", got)
	}
}
