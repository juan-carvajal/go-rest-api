package main

import (
	"github.com/likexian/whois-go"
	"strings"
)


func formatRaw(raw string,lookFor string)(string){
	cosa := strings.Split(raw, "\n")
	var result string
	for _, value := range cosa {
		if strings.HasPrefix(value, lookFor) {
			subs := strings.Split(value, ":")
			var index int
			found := false
			for pos, char := range subs[1] {
				if char != ' ' {
					index = pos
					found = true
					break
				}
			}
			if found == true {
				result = subs[1][index:]
				break
			}
		}
	}
	return result
}



func getWhoIsData(ip string)(string,string, error){
	raw, err := whois.Whois(ip)
	var name string
	var country string
	if err == nil {
		name=formatRaw(raw,"OrgName")
		country=formatRaw(raw,"Country")

	}
	return name,country,err
}