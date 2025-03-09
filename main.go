package main

import (
	"log"
	"maand/cmd"
)

func main() {
	log.Default().SetFlags(log.Lmsgprefix)
	cmd.Execute()
}
