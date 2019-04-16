package main

import (
	"fmt"
	"janus"
	"log"
	"time"
)

func main() {
	cli := janus.NewClient()
	if err := cli.Connect("http://113.105.153.240:9998/janus"); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Session: %d\n", cli.SessionId)
	cli.Run()

	if err := cli.Attach("janus.plugin.pocroom"); err != nil {
		log.Fatal("attach error: ", err)
	}

	fmt.Println("Attach Handle ID: ", cli.HandleId)
	time.Sleep(5 * time.Second)
	if err := cli.Detach(); err != nil {
		log.Fatal("detach error: ", err)
	}

	fmt.Println("Detach ok.")

	if err := cli.Close(); err != nil {
		log.Fatal("close error: ", err)
	}

	cli.Stop()
	fmt.Println("Done.")
}
