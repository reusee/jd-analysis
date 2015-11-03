package main

import (
	"crypto/sha512"
	"fmt"
	"io/ioutil"
	"net/http"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"github.com/PuerkitoBio/goquery"
)

var (
	pt = fmt.Printf
	sp = fmt.Sprintf
)

func getBytes(client *http.Client, url string) ([]byte, error) {
	retry := 3
get:
	resp, err := client.Get(url)
	if err != nil {
		if retry > 0 {
			retry--
			goto get
		} else {
			return nil, me(err, "get")
		}
	}
	defer resp.Body.Close()
	bs, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		if retry > 0 {
			retry--
			goto get
		} else {
			return nil, me(err, "get")
		}
	}
	return bs, nil
}

func getDoc(client *http.Client, url string) (*goquery.Document, error) {
	retry := 3
get:
	resp, err := client.Get(url)
	if err != nil {
		if retry > 0 {
			retry--
			goto get
		} else {
			return nil, me(err, "get")
		}
	}
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		if retry > 0 {
			retry--
			goto get
		} else {
			return nil, me(err, "new document from response")
		}
	}
	return doc, nil
}

func getGbkDoc(client *http.Client, url string) (*goquery.Document, error) {
	retry := 3
get:
	resp, err := client.Get(url)
	if err != nil {
		if retry > 0 {
			retry--
			goto get
		} else {
			return nil, me(err, "get")
		}
	}
	defer resp.Body.Close()
	r := transform.NewReader(resp.Body, simplifiedchinese.GBK.NewDecoder())
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		if retry > 0 {
			retry--
			goto get
		} else {
			return nil, me(err, "new document from response")
		}
	}
	return doc, nil
}

func hash(in string) []byte {
	sum := sha512.Sum512([]byte(in))
	return sum[:]
}
