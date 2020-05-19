package main

import (
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"github.com/badoux/goscraper"
)

func processElement(index int, element *goquery.Selection)(bool) {


	
    value, exists := element.Attr("rel")
    return exists && value=="shortcut icon"
}


func findTitle(domain string)(string,error)  {
	response, err := http.Get("http://"+domain)
	var returnVal string
    if err == nil {
		defer response.Body.Close()
		document, err := goquery.NewDocumentFromReader(response.Body)
		if err == nil {
			returnVal=document.Find("title").First().Text()
			
		}
	}
	return returnVal,err


    
}

func GetHTMLInfo(url string) (string, string,error) {
	url = "http://" + url
	s, err := goscraper.Scrape(url, 5)
	var icon,title string
	if err == nil {
		icon=s.Preview.Icon
		title=s.Preview.Title
	}
	return icon, title,err
}


func findLogo(domain string)(string,error){
	response, err := http.Get("http://"+domain)
	var returnVal string
    if err == nil {
		defer response.Body.Close()
		document, err := goquery.NewDocumentFromReader(response.Body)
		if err == nil {
			returnVal=document.Find("link").FilterFunction(processElement).First().AttrOr("href","")
			
		}
	}
	return returnVal,err
}