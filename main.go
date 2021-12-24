package main

import (
	"fmt"
	"html"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/gorilla/mux"
)

type fileInfo struct {
	os.FileInfo
	url url.URL
}

func (f *fileInfo) FixedName() string {
	if f.IsDir() {
		return fmt.Sprintf("%s/", f.Name())
	}
	return f.Name()
}

func (f *fileInfo) Url() string {
	return f.url.String()
}

type handler struct {
	templates *template.Template
}

func (h *handler) handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		defer r.Body.Close()
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		dir := http.Dir(os.Getenv("HOME"))
		path := html.EscapeString(r.URL.Path)
		f, err := dir.Open(path)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "File not found: %s\n", path)
			return
		}
		stat, err := f.Stat()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if stat.IsDir() {
			files, err := f.Readdir(0)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			sort.Slice(files, func(i, j int) bool {
				if files[i].IsDir() && !files[j].IsDir() {
					return true
				} else if !files[i].IsDir() && files[j].IsDir() {
					return false
				}
				return files[i].Name() < files[j].Name()
			})
			fileInfos := []*fileInfo{}
			for _, info := range files {
				name := info.Name()
				if strings.HasPrefix(name, ".") && r.URL.Query().Get("showHidden") != "true" {
					continue
				}
				if info.IsDir() {
					name = fmt.Sprintf("%s/", name)
				}
				if path != "/" {
					name = fmt.Sprintf("%s%s", path, name)
				}
				file := &fileInfo{
					FileInfo: info,
					url:      url.URL{Path: name},
				}
				fileInfos = append(fileInfos, file)
			}
			h.templates.ExecuteTemplate(w, "directory.html", fileInfos)
		} else {
			size := stat.Size()
			buf, err := io.ReadAll(f)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if n, err := w.Write(buf); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			} else if int64(n) != size {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	default:
		fmt.Fprintf(w, "No\n")
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func main() {
	r := mux.NewRouter()
	templates, err := template.New("templates").ParseGlob("templates/*")
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	h := &handler{
		templates,
	}
	r.PathPrefix("/").HandlerFunc(h.handle)
	go func() { log.Fatal(http.ListenAndServe(":8080", r)) }()
	select {}
}
