package main

import (
	"net/http"
	"net/http/cookiejar"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	log  = logrus.New()
	args = os.Args[1:]
)

type attachments struct {
	ID, Ticket, Filename, ContentType, FileSize string
}

type request struct {
	Client           *http.Client
	Rt, ID, Filename string
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

func main() {
	jar, _ := cookiejar.New(nil)

	client := &http.Client{
		Jar: jar,
	}

	authenticate(client)
	var att []attachments
	for _, ticket := range args {
		att = append(att, getAttachments(client, ticket)...)
		log.Debug(att)
		if len(att) == 0 {
			log.Errorln("Ticket", ticket, "does not exists")
			continue
		}
	}
	processDownload(client, att)
	logout(client)
}
