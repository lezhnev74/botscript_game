package httpapi

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// spaRoutes are the client-side routes of the web app: each serves index.html.
var spaRoutes = map[string]bool{"/": true, "/bot": true, "/leaderboard": true, "/admin": true, "/index.html": true}

const placeholder = `<!doctype html><meta charset="utf-8"><title>Bot Arena</title>
<body style="font-family:sans-serif;background:#111;color:#ddd;padding:2em">
<h1>Bot Arena</h1>
<p>The web app is not built yet: web/dist is missing.</p>
<p>The API works: <code>/api/leaderboard</code>, <code>/ws</code>.</p>
`

// staticHandler serves the built web app from dir.
func staticHandler(dir string) http.Handler {
	files := http.Dir(dir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if spaRoutes[r.URL.Path] {
			serveIndex(w, r, files)
			return
		}
		serveFile(w, r, files)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, files http.FileSystem) {
	if !serve(w, r, files, "index.html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(placeholder))
	}
}

func serveFile(w http.ResponseWriter, r *http.Request, files http.FileSystem) {
	name := path.Clean("/" + r.URL.Path)
	if !strings.HasPrefix(r.URL.Path, "/") || !serve(w, r, files, name) {
		http.NotFound(w, r)
	}
}

// serve writes the named regular file; false if there is none.
func serve(w http.ResponseWriter, r *http.Request, files http.FileSystem, name string) bool {
	f, err := files.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() || !fs.ValidPath(strings.TrimPrefix(path.Clean("/"+name), "/")) {
		return false
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	return true
}
