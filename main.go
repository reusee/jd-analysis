package main

import (
	"bytes"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func main() {
	url := "http://www.jd.com/allSort.aspx"
	doc, err := getGbkDoc(http.DefaultClient, url)
	ce(err, "get all category page")

	clientSet := NewClientSet()

	header := doc.Find("div.mt h2 a").FilterFunction(func(_ int, se *goquery.Selection) bool {
		return se.Text() == "服饰内衣"
	})
	if header.Length() != 1 {
		panic("no such category")
	}

	type Job struct {
		Path []string
		Page int
	}
	jobs := make(chan *Job)

	db, err := sqlx.Connect("mysql", "root:ffffff@tcp(127.0.0.1:3306)/jd?parseTime=true&autocommit=true")
	ce(err, "db")

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		catePattern := regexp.MustCompile(`([0-9]+)-([0-9]+)-([0-9]+)`)
		cateFound := false
		header.ParentsFiltered("div.m").Find("dl dt a").Each(func(_ int, se *goquery.Selection) {
			cateFound = true
			cateName := se.Text()
			pt("%s\n", cateName)
			subcateFound := false
			se.ParentsFiltered("dl").Find("dd em a").Each(func(_ int, se *goquery.Selection) {
				subcateFound = true
				name := se.Text()
				url, ok := se.Attr("href")
				if !ok {
					panic(sp("no url for subcate %s", name))
				}
				ms := catePattern.FindStringSubmatch(url)
				catePath := ms[1:]
				if len(catePath) != 3 {
					panic("invalid category path")
				}
				wg.Add(1)
				jobs <- &Job{
					Path: catePath,
					Page: 1,
				}
			})
			if !subcateFound {
				panic("no subcate found")
			}
		})
		if !cateFound {
			panic("no cate found")
		}
		wg.Done()
	}()

	for i := 0; i < 64; i++ {
		go func() {
			for job := range jobs {
				// url
				url := sp("http://list.jd.com/list.html?cat=%s&page=%d",
					strings.Join(job.Path, ","), job.Page)

				// get bytes
				var bs []byte
				var n int
				err := db.Get(&n, `SELECT COUNT(*) FROM html WHERE url_hash = ?`,
					hash(url))
				if n > 0 {
					err = db.Get(&bs, `SELECT html FROM html WHERE url_hash = ?`,
						hash(url))
					ce(err, "get html")
					pt("from db %s\n", url)
				} else {
					clientSet.Do(func(client *http.Client) ClientState {
						bs, err = getBytes(client, url)
						if err != nil {
							return Bad
						}
						return Good
					})
					pt("from website %s\n", url)
				}

				// get doc
				doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bs))
				ce(err, "new doc")

				// insert into database
				_, err = db.Exec(`REPLACE INTO html (url, url_hash, html) VALUES (?, ?, ?)`,
					url, hash(url), bs)
				ce(err, "insert html")

				retry := func() {
					_, err = db.Exec(`DELETE FROM html WHERE url_hash = ?`, hash(url))
					ce(err, "delete html")
					job := job
					go func() {
						wg.Add(1)
						jobs <- job
						wg.Done()
					}()
				}

				// find items
				itemFound := false
				doc.Find("li.gl-item").Each(func(_ int, se *goquery.Selection) {
					itemFound = true
				})
				if !itemFound {
					// retry
					retry()
					continue
				}

				// next job
				maxPage, err := strconv.Atoi(doc.Find("span.p-skip em b").Text())
				if err != nil {
					retry()
					continue
				}
				pt("%d %d\n", job.Page, maxPage)
				if job.Page == maxPage {
					// no more
					wg.Done()
				} else {
					// next page
					job := job
					go func() {
						wg.Add(1)
						job.Page++
						jobs <- job
						wg.Done()
					}()
				}

			}
		}()
	}

	wg.Wait()
	close(jobs)
}
