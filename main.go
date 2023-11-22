package main

import (
	"log"
	"os"

	"github.com/alecthomas/kong"
)

type CLI struct {
	SrcURL     string `name:"src" short:"s" env:"SRC_URL" required:"true" help:"Source repo URL"`
	DstURL     string `name:"dst" short:"d" env:"DST_URL" required:"true" help:"Destination repo URL"`
	SrcToken   string `name:"src-token" short:"t" env:"SRC_TOKEN" help:"Source repo token/password"`
	DstToken   string `name:"dst-token" short:"e" env:"DST_TOKEN" help:"Destination repo token/password"`
	SrcUser    string `name:"src-user" short:"u" env:"SRC_USER" help:"Source repo user"`
	DstUser    string `name:"dst-user" short:"v" env:"DST_USER" help:"Destination repo user"`
	Verbose    bool   `name:"verbose" env:"VERBOSE" help:"Enable verbose output"`
	SrcHeadRef string `name:"src-head-ref" short:"b" env:"SRC_HEAD_REF" help:"Source repo head ref"`
}

func main() {
	var cli CLI
	kong.Parse(&cli)

	if cli.Verbose {
		log.SetFlags(log.Lshortfile | log.Lmicroseconds)
	} else {
		log.SetFlags(0)
	}

	if err := pullPushRepo(cli.SrcURL, cli.SrcUser, cli.SrcToken, cli.DstURL, cli.DstUser, cli.DstToken, cli.SrcHeadRef, cli.Verbose); err != nil {
		log.Println("Error:", err)
		os.Exit(1)
	}
}
