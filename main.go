package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"gopkg.in/cheggaaa/pb.v1"
)

type JsonURL struct {
	u *url.URL
}

func (j *JsonURL) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.u.String())
}

func (j *JsonURL) UnmarshalJSON(b []byte) (err error) {
	var s string
	err = json.Unmarshal(b, &s)
	if err != nil {
		return
	}

	j.u, err = url.Parse(s)
	return
}

func (j JsonURL) String() string {
	return j.u.String()
}

type BookEssential struct {
	Epoch string `json:"epoch,omitempty"`
	Kind  string `json:"kind,omitempty"`
	Genre string `json:"genre,omitempty"`

	Url  JsonURL `json:"url"`  // human readable page
	Href JsonURL `json:"href"` // further API details
	Slug string  `json:"slug"`

	Author string `json:"author"`
	Title  string `json:"title"`
}

type Tag struct {
	Url  JsonURL `json:"url"`  // human readable page
	Href JsonURL `json:"href"` // further API details
	Name string  `json:"name"`
	Slug string  `json:"slug"`
}

type BookDetails struct {
	Authors []Tag `json:"authors"`
	Epochs  []Tag `json:"epochs"`
	Kinds   []Tag `json:"kinds"`
	Genres  []Tag `json:"genres"`

	Slug     string          `json:"slug"`
	Title    string          `json:"title"`
	Parent   *BookEssential  `json:"parent,omitempty"`
	Children []BookEssential `json:"children,omitempty"`
	URL      JsonURL         `json:"url"` // human readable page

	Txt  JsonURL `json:"txt,omitempty"`
	Xml  JsonURL `json:"xml,omitempty"`
	Html JsonURL `json:"html,omitempty"`
	Fb2  JsonURL `json:"fb2,omitempty"`
	Epub JsonURL `json:"epub,omitempty"`
	Mobi JsonURL `json:"mobi,omitempty"`
	Pdf  JsonURL `json:"pdf,omitempty"`

	// TODO: add other side files
}

func mustParseUrl(str string) (u *url.URL) {
	var err error
	u, err = url.Parse(str)
	if err != nil {
		panic("predefined url cannot be parsed")
	}
	return u
}

var (
	httpClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:    10,
			IdleConnTimeout: 30 * time.Second,
		},
	}
	Offline     = flag.Bool("offline", false, "don't download anything from origin")
	ErrOffline  = errors.New("resource unavailable: offline flag specified")
	BooksFile   = "books.json"
	ApiBooksUrl = mustParseUrl("https://wolnelektury.pl/api/books/")
	DetailsFile = "details.json"
)

func cachedFile(filePath string, originUrl *url.URL) (content []byte, err error) {
	// TODO: redownload at some chance

	content, err = ioutil.ReadFile(filePath)
	if err == nil {
		return
	}

	if !os.IsNotExist(err) {
		return
	}

	if *Offline {
		return nil, ErrOffline
	}
	log.Print(filePath, " not available offline, downloading")

	var resp *http.Response
	resp, err = httpClient.Get(originUrl.String())
	if err != nil {
		return
	}
	defer resp.Body.Close()
	content, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	err = ioutil.WriteFile(filePath, content, 0644)
	log.Print(filePath, " synced and saved")
	return
}

func BooksList() (books []BookEssential, err error) {
	var content []byte
	content, err = cachedFile(BooksFile, ApiBooksUrl)
	if err != nil {
		return
	}
	err = json.Unmarshal(content, &books)
	return
}

func (b BookEssential) Details() (book BookDetails, err error) {
	defer func() { book.Slug = b.Slug }()

	var content []byte
	content, err = cachedFile(path.Join(b.Slug, DetailsFile), b.Href.u)
	if err != nil {
		return
	}

	err = json.Unmarshal(content, &book)
	return
}

func (b BookEssential) ObtainBook() (book BookDetails) {
	err := os.Mkdir(b.Slug, 0755)
	if err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}

	book, err = b.Details()
	if err != nil {
		log.Fatal(err)
	}

	for fileName, originUrl := range book.Files() {
		_, err = cachedFile(path.Join(b.Slug, fileName), originUrl.u)
		if err != nil {
			log.Fatal("failed to obtain file ", fileName, err)
		}
	}

	return
}

func (b BookDetails) Files() (f map[string]JsonURL) {
	f = make(map[string]JsonURL)
	if b.Txt.String() != "" {
		f[b.Slug+".txt"] = b.Txt
	}
	if b.Xml.String() != "" {
		f[b.Slug+".xml"] = b.Xml
	}
	if b.Html.String() != "" {
		f[b.Slug+".html"] = b.Html
	}
	if b.Fb2.String() != "" {
		f[b.Slug+".fb2"] = b.Fb2
	}
	if b.Epub.String() != "" {
		f[b.Slug+".epub"] = b.Epub
	}
	if b.Mobi.String() != "" {
		f[b.Slug+".mobi"] = b.Mobi
	}
	if b.Pdf.String() != "" {
		f[b.Slug+".pdf"] = b.Pdf
	}
	// TODO: add other side files

	return
}

func main() {
	flag.Parse()

	books, err := BooksList()
	if err != nil {
		log.Fatal(err)
	}

	progress := pb.StartNew(len(books))
	for _, bookBase := range books {
		bookBase.ObtainBook()
		progress.Increment()
	}
	progress.Finish()
}
