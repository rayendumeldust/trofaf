package main

import (
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"

	"github.com/eknkc/amber"
	"github.com/krautchan/gbt/module/api/rss"
)

// TODO : All fatal errors should be non-stopping errors when generating the site. Allows
// for corrections of the code, then re-triggering the generation.
// TODO : Check for rss template, if present generate RSS feed

var (
	postTpl   *template.Template
	postTplNm = "post.amber"
	rssTplNm  = "rss.amber"
)

type sortableFileInfo []os.FileInfo

// TODO : Should sort on pubtime of posts instead
func (s sortableFileInfo) Len() int           { return len(s) }
func (s sortableFileInfo) Less(i, j int) bool { return s[i].ModTime().Before(s[j].ModTime()) }
func (s sortableFileInfo) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func FilterDir(s sortableFileInfo) sortableFileInfo {
	for i := 0; i < len(s); {
		if s[i].IsDir() {
			s[i], s = s[len(s)-1], s[:len(s)-1]
		} else {
			i++
		}
	}
	return s
}

func compileTemplate() {
	ap := filepath.Join(TemplatesDir, postTplNm)
	if _, err := os.Stat(ap); os.IsNotExist(err) {
		// Amber post template does not exist, compile the native Go templates
		postTpl, err = template.ParseGlob(filepath.Join(TemplatesDir, "*.html"))
		if err != nil {
			log.Fatal("FATAL ", err)
		}
		postTplNm = "post" // TODO : Validate this...
	} else {
		c := amber.New()
		if err := c.ParseFile(ap); err != nil {
			log.Fatal("FATAL ", err)
		}
		if postTpl, err = c.Compile(); err != nil {
			log.Fatal("FATAL ", err)
		}
	}
}

func generateSite() {
	// First compile the template(s)
	compileTemplate()
	// Clear the public directory, except subdirs
	fis, err := ioutil.ReadDir(PublicDir)
	if err != nil {
		log.Fatal("FATAL ", err)
	}
	for _, fi := range fis {
		if !fi.IsDir() && fi.Name() != "favicon.ico" {
			err = os.Remove(filepath.Join(PublicDir, fi.Name()))
			if err != nil {
				log.Println("DELETE ERROR ", err)
			}
		}
	}
	// Now read the posts
	fis, err = ioutil.ReadDir(PostsDir)
	if err != nil {
		log.Fatal("FATAL ", err)
	}
	sfi := sortableFileInfo(fis)
	sfi = FilterDir(sfi)
	sort.Reverse(sfi)

	recent := make([]*ShortPost, Options.RecentPostsCount)
	all := make([]*LongPost, len(sfi))
	// First pass to get the recent posts (and others) so that
	// they can be passed to all posts.
	for i, fi := range sfi {
		all[i] = newLongPost(fi)
		if i < Options.RecentPostsCount {
			recent[i] = all[i].Short()
		}
	}

	for i, p := range all {
		td := newTemplateData(p, i, recent, all)
		generateFile(td, i == 0)
	}
	td := newTemplateData(nil, 0, recent, nil)
	if err := generateRss(td); err != nil {
		log.Fatal("FATAL ", err)
	}
}

func generateRss(td *TemplateData) error {
	r := rss.New(td.SiteName, "", Options.BaseURL)
	base, err := url.Parse(Options.BaseURL)
	if err != nil {
		return err
	}
	for _, p := range td.Recent {
		u, err := base.Parse(p.Slug)
		if err != nil {
			return err
		}
		r.AddItem(p.Title, u.String(), p.Description, p.Author, "")
	}
	return r.WriteToFile(filepath.Join(PublicDir, "rss"))
}

func generateFile(td *TemplateData, idx bool) {
	var w io.Writer

	fw, err := os.Create(filepath.Join(PublicDir, td.Post.Slug))
	if err != nil {
		log.Fatal("FATAL ", err)
	}
	defer fw.Close()
	w = fw
	if idx {
		idxw, err := os.Create(filepath.Join(PublicDir, "index.html"))
		if err != nil {
			log.Fatal("FATAL ", err)
		}
		defer idxw.Close()
		w = io.MultiWriter(fw, idxw)
	}
	err = postTpl.ExecuteTemplate(w, postTplNm, td)
	if err != nil {
		log.Fatal("FATAL ", err)
	}
}
