package main

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/buaazp/fasthttprouter"
	"github.com/valyala/fasthttp"

	"io/ioutil"
	"log"
	"net/http"
	"time"

	_ "github.com/lib/pq"
)

type ServerInfo struct {
	Address  string `json:"address,omitempty"`
	SSLGrade string `json:"ssl_grade,omitempty"`
	Country  string `json:"country,omitempty"`
	Owner    string `json:"owner,omitempty"`
}

type QueryResult struct {
	Servers          []ServerInfo `json:"servers,omitempty"`
	ServersChanged   bool         `json:"servers_changed,omitempty"`
	PreviousSSLGrade string       `json:"previous_ssl_grade,omitempty"`
	Logo             string       `json:"logo,omitempty"`
	Title            string       `json:"title,omitempty"`
	IsDown           bool         `json:"is_down,omitempty"`
	SSLGrade         string       `json:"ssl_grade,omitempty"`
}

type QueryHistory struct {
	Items map[string]QueryResult `json:"items,omitempty"`
}

func dumpMap(space string, m map[string]interface{}) {
	for k, v := range m {
		if mv, ok := v.(map[string]interface{}); ok {
			fmt.Printf("{ \"%v\": \n", k)
			dumpMap(space+"\t", mv)
			fmt.Printf("}\n")
		} else {
			fmt.Printf("%v %v : %v\n", space, k, v)
		}
	}
}

func historyReport(ctx *fasthttp.RequestCtx) {

	db, err := sql.Open("postgres",
		"postgresql://root@DESKTOP-F7UV418:26257?sslmode=disable")
	if err != nil {
		ctx.Error("Can not connect to the database", fasthttp.StatusInternalServerError)
	}
	rows, err := db.Query("SELECT domain,time,result FROM tests")
	if err != nil {
		ctx.Error("Error retrieving database information", fasthttp.StatusInternalServerError)
	}
	defer rows.Close()
	history := &QueryHistory{}
	queries := make(map[string]QueryResult)
	for rows.Next() {
		var testTime time.Time
		var domain string
		var result string
		if err := rows.Scan(&domain, &testTime, &result); err != nil {
			ctx.Error("Error parsing database fields", fasthttp.StatusInternalServerError)
		}
		res := &QueryResult{}
		json.Unmarshal([]byte(result), &res)
		queries[domain] = *res

	}
	history.Items = queries
	serialized, err := json.Marshal(history)
	if err != nil {
		ctx.Error("Unable to serialize data", fasthttp.StatusInternalServerError)
	}

	fmt.Fprint(ctx, string(serialized))
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
}

func singleReport(ctx *fasthttp.RequestCtx) {

	qdomain := ctx.UserValue("domain").(string)

	db, err := sql.Open("postgres",
		"postgresql://root@DESKTOP-F7UV418:26257?sslmode=disable")
	if err != nil {
		ctx.Error("Can not connect to the database", fasthttp.StatusInternalServerError)
		return
	}
	defer db.Close()
	resp, err := http.Get("https://api.ssllabs.com/api/v3/analyze?host=" + qdomain + "&fromCache=on&maxAge=1")
	if err != nil {
		ctx.Error("Can not read data from SSL analysis provider", fasthttp.StatusInternalServerError)
	}
	var dat map[string]interface{}
	defer resp.Body.Close()
	responseData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ctx.Error("Can not read data from SSL analysis provider", fasthttp.StatusInternalServerError)
	}
	json.Unmarshal(responseData, &dat)
	status, ok := dat["status"].(string)
	if !ok {
		status = "ERROR"
	}
	fmt.Println("Querying for ", qdomain, " Status:", status)
	for status != "READY" && status != "ERROR" {
		resp, err = http.Get("https://api.ssllabs.com/api/v3/analyze?host=" + qdomain)

		if err != nil {
			ctx.Error("Can not read data from SSL analysis provider", fasthttp.StatusInternalServerError)
		}

		dat = make(map[string]interface{})
		defer resp.Body.Close()
		responseData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			ctx.Error("Can not read data from SSL analysis provider", fasthttp.StatusInternalServerError)
		}
		json.Unmarshal(responseData, &dat)
		status = dat["status"].(string)
		fmt.Println("Querying for ", qdomain, " Status:", status)
		if status == "ERROR" {
			break
		}
		time.Sleep(5 * time.Second)
	}

	if status == "ERROR" {
		status, ok = dat["statusMessage"].(string)
		if !ok {
			status = "Unknown error"
		}
		ctx.Error(status, fasthttp.StatusInternalServerError)
		response := &QueryResult{}
		response.IsDown = true
		serialized, err := json.Marshal(response)
		if err != nil {
			ctx.Error("Unable to serialize data", fasthttp.StatusInternalServerError)
		}
		fmt.Fprint(ctx, string(serialized))
	} else {
		rows, err := db.Query("SELECT domain,time,result FROM tests where domain='" + qdomain + "'")
		if err != nil {
			ctx.Error("Error retrieving database information", fasthttp.StatusInternalServerError)
		}
		defer rows.Close()
		var testTime time.Time
		var domain string
		var result string
		exist := false
		for rows.Next() {

			if err := rows.Scan(&domain, &testTime, &result); err != nil {
				ctx.Error("Error parsing database fields", fasthttp.StatusInternalServerError)
			}
			exist = true
			break
		}
		currentTest := &QueryResult{}
		servers := make([]ServerInfo, 0)
		endpointSlice := dat["endpoints"].([]interface{})

		for _, endpoint := range endpointSlice {
			server := &ServerInfo{}
			server.Address = endpoint.(map[string]interface{})["ipAddress"].(string)
			sslg, ok := endpoint.(map[string]interface{})["grade"].(string)
			if ok {
				server.SSLGrade = sslg
			}

			name, country, err := getWhoIsData(server.Address)
			if err == nil {
				server.Country = country
				server.Owner = name
			} else {
				log.Print(err)
			}
			servers = append(servers, *server)
		}
		currentTest.Servers = servers
		logo, title, err := GetHTMLInfo(qdomain)

		if err == nil {
			currentTest.Title = title
			currentTest.Logo = logo
		}

		bestGrade := "Z"
		for _, m := range servers {
			if m.SSLGrade != "" && m.SSLGrade < bestGrade {
				bestGrade = m.SSLGrade
			}
		}
		if bestGrade != "Z" {
			currentTest.SSLGrade = bestGrade
		} else {
			currentTest.SSLGrade = "Undetermined"
		}
		currentTest.ServersChanged = false
		if exist {
			oldRes := &QueryResult{}
			json.Unmarshal([]byte(result), oldRes)
			currentTest.PreviousSSLGrade = oldRes.SSLGrade
			if oldRes.SSLGrade != currentTest.SSLGrade {
				currentTest.ServersChanged = true
			}
			serialized, err := json.Marshal(currentTest)
			if err != nil {
				ctx.Error("Unable to serialize data", fasthttp.StatusInternalServerError)
			}
			query := "update tests set time=now(), result='" + string(serialized) + "' where domain='" + qdomain + "'"
			if _, err := db.Exec(query); err != nil {
				ctx.Error("Unable to update new test", fasthttp.StatusInternalServerError)
			}
			fmt.Fprint(ctx, string(serialized))
		} else {
			serialized, err := json.Marshal(currentTest)
			if err != nil {
				ctx.Error("Unable to serialize data", fasthttp.StatusInternalServerError)
			}
			query := "insert into tests values (now(),'" + qdomain + "','" + string(serialized) + "')"
			if _, err := db.Exec(query); err != nil {
				log.Print(err)
				ctx.Error("Unable to insert test result", fasthttp.StatusInternalServerError)
			}

			fmt.Fprint(ctx, string(serialized))
		}

	}
	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
	ctx.Response.Header.Set("Content-Type", "application/json")

}

func main() {

	db, err := sql.Open("postgres",
		"postgresql://root@DESKTOP-F7UV418:26257?sslmode=disable")
	if err != nil {
		log.Fatal("error connecting to the database: ", err)
	}

	if _, err := db.Exec(
		"CREATE TABLE IF NOT EXISTS tests (time TIMESTAMP not null,domain STRING not null , result JSONB not null,PRIMARY KEY (domain))"); err != nil {
		log.Fatal(err)
	}

	router := fasthttprouter.New()
	router.GET("/report/:domain", singleReport)
	router.GET("/history", historyReport)
	log.Fatal(fasthttp.ListenAndServe(":8081", router.Handler))

}
