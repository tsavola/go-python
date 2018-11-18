package main

import (
	"time"

	"github.com/tsavola/go-python"
)

// main runs two Python threads.
func main() {
	turtled := make(chan struct{})

	go func() {
		defer close(turtled)
		turtle()
	}()

	spinned := make(chan struct{})

	go func() {
		defer close(spinned)
		spin(turtled)
	}()

	<-spinned
}

// turtle draws something in a window.
func turtle() {
	thread := python.NewThread()
	defer thread.Close()

	module, err := python.Import(thread, "turtle")
	if err != nil {
		panic(err)
	}

	screen, err := module.Call(thread, "Screen")
	if err != nil {
		panic(err)
	}

	turtle, err := module.Call(thread, "Turtle")
	if err != nil {
		panic(err)
	}

	turtle.Call(thread, "forward", 50)
	turtle.Call(thread, "left", 90)
	turtle.Call(thread, "foward", 30)

	screen.Call(thread, "exitonclick")
}

// spin rotates a bar in the console.
func spin(quit <-chan struct{}) {
	const spinner = "-\\|/"

	thread := python.NewThread()
	defer thread.Close()

	sys, err := python.Import(thread, "sys")
	if err != nil {
		panic(err)
	}

	stdout, err := sys.Attr(thread, "stdout")
	if err != nil {
		panic(err)
	}

	writeToStdout, err := stdout.Attr(thread, "write")
	if err != nil {
		panic(err)
	}

	flushStdout, err := stdout.Attr(thread, "flush")
	if err != nil {
		panic(err)
	}

	defer writeToStdout.Invoke(thread, "\n")

	ticker := time.NewTicker(time.Second / 10)

	for i := 0; ; i++ {
		select {
		case <-ticker.C:
			writeToStdout.Invoke(thread, string(spinner[i%len(spinner)])+"\r")
			flushStdout.Invoke(thread)

		case <-quit:
			return
		}
	}
}
