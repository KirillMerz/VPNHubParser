package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	URL                = "https://vpnhub.me/en/all-open-vpn-list.html"
	configurationsPath = "configurations"
)

var (
	commentsPattern   = regexp.MustCompile(`#(.*)`)
	blankLinesPattern = regexp.MustCompile(`[\n,\r]{2,}`)
)

func main() {
	// Create configurations directory if it isn't exists
	if _, err := os.Stat(configurationsPath); os.IsNotExist(err) {
		err := os.Mkdir(configurationsPath, 0777)
		if err != nil {
			log.Fatalln(err)
		}
	}

	pagesWG := sync.WaitGroup{}
	for i := 1; i < 101; i++ {
		pagesWG.Add(1)
		go func(i int) {
			defer pagesWG.Done()
			collectConfigurations(i)
		}(i)
	}
	pagesWG.Wait()
}

func collectConfigurations(page int) {
	configurationsWG := sync.WaitGroup{}

	request, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		log.Fatal(err)
	}
	request.AddCookie(&http.Cookie{Name: "page", Value: strconv.Itoa(page)})

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		log.Fatalln(err)
	}
	defer response.Body.Close()

	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	document.Find("tbody").Find("tr").Each(func(_ int, tr *goquery.Selection) {
		configurationsWG.Add(1)
		go func() {
			defer configurationsWG.Done()
			downloadConfiguration(parseTableRow(tr))
		}()
	})

	configurationsWG.Wait()
}

func parseTableRow(tr *goquery.Selection) Configuration {
	i := tr.Find("i.fas.fa-download")
	country := tr.Find("a").Text()
	hostname, _ := i.Attr("hostname")
	connType, _ := i.Attr("type")
	port, _ := i.Attr("port")
	hid, _ := i.Attr("hid")
	return Configuration{country, hostname, connType, port, hid}
}

func generateConfigurationURL(conf Configuration) string {
	return fmt.Sprintf(
		"http://www.vpngate.net/common/openvpn_download.aspx?hostname=%s&type=%s&port=%s&hid=%s&sid=%s000",
		conf.hostname, conf.connType, conf.port, conf.hid, strconv.FormatInt(time.Now().Unix(), 10),
	)
}

func downloadConfiguration(conf Configuration) {
	response, err := http.Get(generateConfigurationURL(conf))
	if err != nil {
		return
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return
	}

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return
	}

	countryPath := configurationsPath + "/" + conf.country

	// Create country directory if it isn't exists
	if _, err := os.Stat(countryPath); os.IsNotExist(err) {
		os.Mkdir(countryPath, 0777)
	}

	filePath := countryPath + "/" + conf.hostname + ".ovpn"

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return
	}
	defer file.Close()

	_, err = file.Write(minifyConfiguration(data))
	if err != nil {
		return
	}
}

func minifyConfiguration(conf []byte) []byte {
	conf = commentsPattern.ReplaceAll(conf, []byte{})
	conf = blankLinesPattern.ReplaceAll(conf, []byte{'\n'})
	return conf
}
