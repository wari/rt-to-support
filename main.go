package main

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	log  = logrus.New()
	args = os.Args[1:]
)

type attachments struct {
	ID, Filename, ContentType, FileSize string
}

func authenticate(client *http.Client) {
	form := url.Values{}
	form.Add("user", viper.GetString("user"))
	form.Add("pass", viper.GetString("pass"))

	baseURL := viper.GetString("rt") + viper.GetString("restURL")
	log.Warnln("Logging In", form.Encode())
	resp, err := client.PostForm(baseURL, form)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	log.Info("Logged In")

	reply, err := ioutil.ReadAll(resp.Body)
	if !strings.Contains(string(reply), "200 Ok") {
		log.Fatal(string(reply))
	} else {
		log.Println("Login Successful")
	}
}

func getAttachments(client *http.Client, rt string) []attachments {
	baseURL := viper.GetString("rt") + viper.GetString("restURL")
	var att []attachments
	resp, err := client.Get(baseURL + "/ticket/" + rt + "/attachments")
	if err != nil {
		log.Fatalln(err)
	}

	defer resp.Body.Close()
	// There are no error conditions, just zer0 attachments
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		f := false
		s := scanner.Text()
		if strings.HasPrefix(s, "Attachments: ") {
			s = strings.TrimPrefix(s, "Attachments: ")
			f = true
		}
		if strings.HasPrefix(s, "             ") {
			s = strings.TrimPrefix(s, "             ")
			f = true
		}
		// String parsing to get the meta data
		if f {
			atts := strings.SplitN(s, ":", 2)
			id, s := atts[0], atts[1]
			s = strings.TrimSuffix(s, ",")
			s = strings.TrimSuffix(s, ")")
			idx := strings.LastIndex(s, "(")
			fn := strings.TrimSpace(s[:idx])
			contents := strings.SplitN(s[idx+1:], " / ", 2)
			att = append(att, attachments{
				ID:          id,
				Filename:    fn,
				ContentType: contents[0],
				FileSize:    contents[1],
			})
		}
	}
	return att
}

type request struct {
	Client           *http.Client
	Rt, ID, Filename string
}

func worker(id int, jobs <-chan request, result chan<- bool) {
	for j := range jobs {
		log.Debugln("Worker", id, "working on", j.ID)
		downloadFile(j.Client, j.Rt, j.ID, j.Filename)
		result <- true
		log.Debugln("Worker", id, "done on", j.ID)
	}
}

func downloadFile(client *http.Client, rt, id, filename string) {
	baseURL := viper.GetString("rt") + viper.GetString("restURL")
	path := "rt/" + rt[:2] + "/" + rt[:3] + "/" + rt + "/attachments/"
	dest := path + filename
	if _, err := os.Stat(dest); err == nil {
		log.Warn("Skipping:   ", dest)
		return
	}

	log.Info("Downloading:", dest)
	err := os.MkdirAll(path, 0775) // no-op if path already exists
	if err != nil {
		log.Fatal(err)
	}

	download(client, dest, baseURL+"/ticket/"+rt+"/attachments/"+id+"/content")
}

func download(client *http.Client, dest, url string) {
	resp, err := client.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	buf := make([]byte, 17)
	_, err = io.ReadAtLeast(resp.Body, buf, 17) // If result is OK, we will consume 17 bytes.
	if err != nil {
		log.Errorln("Unable to read response", err)
		return
	}
	if !bytes.Contains(buf, []byte("Ok\n\n")) {
		log.Error("Error downloading file")
		log.Error(string(buf))
		log.Fatal(buf)
	}

	f, err := os.Create(dest)
	if err != nil {
		log.Errorln(err)
		return
	}

	// Just in case the web fails
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(dest)
		log.Errorln("Unable to download", err)
		return
	}
	f.Close()
	log.Info("Download   :", dest, "...done")
}

func logout(client *http.Client) {
	baseURL := viper.GetString("rt") + viper.GetString("restURL")
	log.Warn("Logging out")
	resp, err := client.Post(baseURL+"/logout",
		"application/x-www-form-urlencoded", bytes.NewBuffer([]byte(``)))
	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()
	log.Info("Logged out")
}

func init() {
	viper.SetConfigName("rt")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/Support")

	viper.SetDefault("rt", "https://demo.bestpractical.com")
	viper.SetDefault("restURL", "/REST/1.0")
	viper.SetDefault("user", "guest")
	viper.SetDefault("pass", "guest")
	viper.SetDefault("workers", 4)

	if err := viper.ReadInConfig(); err != nil { // Handle errors reading the config file
		log.Fatalf("Fatal error config file: %s \n", err)
	}
	if len(args) < 1 {
		log.Fatalln("Please provide the RT ticket numbers", args)
		os.Exit(1)
	}
}

func processDownload(client *http.Client, ticket string, att []attachments) {
	jobs := make(chan request, len(att))
	result := make(chan bool, len(att))

	for f := 0; f <= viper.GetInt("workers"); f++ {
		log.Debugln("Launching worker", f)
		go worker(f, jobs, result)
	}
	count := 0
	for _, a := range att {
		if a.FileSize == "0b" {
			continue
		}
		if a.ContentType == "text/plain" && a.Filename == "(Unnamed)" {
			jobs <- request{client, ticket, a.ID, ticket + "_" + a.ID + ".txt"}
			count++
			continue
		}
		if a.ContentType == "text/html" && a.Filename == "(Unnamed)" {
			jobs <- request{client, ticket, a.ID, ticket + "_" + a.ID + ".html"}
			count++
			continue
		}
		if a.Filename != "(Unnamed)" {
			jobs <- request{client, ticket, a.ID, ticket + "_" + a.ID + "_" + a.Filename}
			count++
			continue
		}
		log.Println("Don't know how to handle:", a.ID, a.ContentType)
	}
	close(jobs)
	for f := 0; f < count; f++ {
		<-result
	}
	close(result)
}

func main() {
	jar, _ := cookiejar.New(nil)

	client := &http.Client{
		Jar: jar,
	}

	authenticate(client)
	for _, ticket := range args {
		att := getAttachments(client, ticket)
		log.Debug(att)
		if len(att) == 0 {
			log.Errorln("Ticket", ticket, "does not exists")
			continue
		}
		processDownload(client, ticket, att)
	}
	logout(client)
}
