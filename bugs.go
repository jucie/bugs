/*
	This is a simple issue tracker for small projects.
	It keeps all data stored at a XML file named bugs.mxl .
*/
package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"text/template"
	"time"
)

const configFile = "bugs.xml"

var db Db

type Change struct {
	XMLName xml.Name  `xml:"change"`
	When    time.Time `xml:"when"`
	Who     string    `xml:"who"`
	Status  int       `xml:"status"`
	Comment string    `xml:"comment"`
}

type Bug struct {
	XMLName xml.Name `xml:"bug"`
	Id      int      `xml:"id"`
	Subject string   `xml:"subject"`
	Changes []Change `xml:"change"`
	Last    *Change  `xml:"-"`
}

type Part struct {
	XMLName xml.Name `xml:"part"`
	Name    string   `xml:"name"`
	Id      int      `xml:"id"`
	Bugs    []Bug    `xml:"bug"`
}

type User struct {
	XMLName xml.Name `xml:"user"`
	Name    string   `xml:"name"`
	Address string   `xml:"address"`
}

type Db struct {
	XMLName xml.Name `xml:"db"`
	NextId  int      `xml:"nextId"`
	Users   []User   `xml:"user"`
	Parts   []Part   `xml:"part"`
}

func (b *Bug) setLast() {
	size := len(b.Changes)
	if size > 0 {
		b.Last = &(b.Changes[size-1])
	}
}

func (p *Part) setLast() {
	for i := 0; i < len(p.Bugs); i++ {
		b := &(p.Bugs[i])
		b.setLast()
	}
}

func (db *Db) setLast() {
	for _, p := range db.Parts {
		p.setLast()
	}
}

func (db *Db) load() {
	xmlFile, err := os.Open(configFile)
	if err != nil {
		panic(err)
	}
	defer xmlFile.Close()

	b, _ := ioutil.ReadAll(xmlFile)
	xml.Unmarshal(b, db)
	db.setLast()
}

func (db *Db) save() {
	const tempFile = "bugs.tmp"
	xmlFile, err := os.Create(tempFile)
	if err != nil {
		panic(err)
	}

	fmt.Fprintf(xmlFile, xml.Header)
	b, err := xml.MarshalIndent(db, "", "\t")
	if err != nil {
		panic(err)
	}
	xmlFile.Write(b)
	xmlFile.Close()
	oldFile := configFile + ".old"
	os.Remove(oldFile)
	os.Rename(configFile, oldFile)
	os.Rename(tempFile, configFile)
}

func findBug(id int) *Bug {
	for _, p := range db.Parts {
		for i, b := range p.Bugs {
			if b.Id == id {
				return &p.Bugs[i]
			}
		}
	}
	return nil
}

func findPart(id int) *Part {
	for i, p := range db.Parts {
		if p.Id == id {
			return &db.Parts[i]
		}
	}
	return nil
}

func loadTemplate(filename string) *template.Template {
	f, err := os.Open("tmpl/" + filename + ".template")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	b, _ := ioutil.ReadAll(f)
	tmpl, err := template.New(filename).Parse(string(b))
	if err != nil {
		panic(err)
	}
	return tmpl
}

func handlerRoot(w http.ResponseWriter, r *http.Request) {
	tmpl := loadTemplate("root")
	err := tmpl.Execute(w, db)
	if err != nil {
		panic(err)
	}
}

func handlerBug(w http.ResponseWriter, r *http.Request) {
	s := r.FormValue("id")
	if s == "" {
		return
	}
	id, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}

	b := findBug(id)
	if b == nil {
		return
	}
	type T struct {
		Users []User
		Bug   *Bug
	}
	var t T
	t.Bug = b
	t.Users = db.Users

	tmpl := loadTemplate("bug")
	err = tmpl.Execute(w, t)
	if err != nil {
		panic(err)
	}
}

func handlerChange1(w http.ResponseWriter, r *http.Request, id int) {
	b := findBug(id)
	if b == nil {
		return
	}
	var c Change
	c.When = time.Now()
	subject := r.FormValue("subject")
	if subject == "" {
		return
	}
	b.Subject = subject
	var err error
	c.Status, err = strconv.Atoi(r.FormValue("status"))
	if err != nil {
		panic(err)
	}

	c.Who = r.FormValue("who")
	c.Comment = r.FormValue("comment")
	b.Changes = append(b.Changes, c)
	b.setLast()

	db.save()
}

func handlerChange(w http.ResponseWriter, r *http.Request) {
	sId := r.FormValue("id")
	if sId == "" {
		return
	}
	id, err := strconv.Atoi(sId)
	if err != nil {
		panic(err)
	}
	handlerChange1(w, r, id)
	http.Redirect(w, r, "/bug?id="+sId, http.StatusSeeOther)
}

func handlerNewBug(w http.ResponseWriter, r *http.Request) {
	sId := r.FormValue("partId")
	if sId == "" {
		return
	}
	id, err := strconv.Atoi(sId)
	if err != nil {
		panic(err)
	}
	p := findPart(id)

	var b Bug
	b.Subject = r.FormValue("subject")
	b.Id = db.NextId
	db.NextId++
	p.Bugs = append(p.Bugs, b)

	db.setLast()
	http.Redirect(w, r, "/bug?id="+strconv.Itoa(b.Id), http.StatusSeeOther)
}

func main() {
	db.load()
	http.HandleFunc("/", handlerRoot)
	http.HandleFunc("/bug", handlerBug)
	http.HandleFunc("/change", handlerChange)
	http.HandleFunc("/newBug", handlerNewBug)
	http.Handle("/static/", http.StripPrefix("/", http.FileServer(http.Dir("."))))
	http.ListenAndServe(":8080", nil)
}
