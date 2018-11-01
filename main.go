package main

import (
	"log"
	"os"

	"github.com/alecthomas/kingpin"
)

func main() {
	srcURL := kingpin.Flag("src", "Source repo URL").Short('s').Required().String()
	dstURL := kingpin.Flag("dst", "Destination repo URL").Short('d').Required().String()
	srcToken := kingpin.Flag("src-token", "Source repo token/password").Short('t').String()
	dstToken := kingpin.Flag("dst-token", "Destination repo token/password").Short('e').String()
	srcUser := kingpin.Flag("src-user", "Source repo user").Short('u').String()
	dstUser := kingpin.Flag("dst-user", "Destination repo user").Short('v').String()
	verbose := kingpin.Flag("verbose", "Enable verbose output").Bool()
	kingpin.Parse()

	if *verbose {
		log.SetFlags(log.Lshortfile | log.Lmicroseconds)
	} else {
		log.SetFlags(0)
	}

	if err := pullPushRepo(*srcURL, *srcUser, *srcToken, *dstURL, *dstUser, *dstToken, *verbose); err != nil {
		log.Println("Error:", err)
		os.Exit(1)
	}
}
