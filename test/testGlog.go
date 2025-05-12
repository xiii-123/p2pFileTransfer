package main

import "github.com/sirupsen/logrus"

func main() {
	var log = logrus.New()
	log.Printf("Hello, %s! Welcome to the interactive CLI!\n", "world")
}
