// Copyright 2015 TVersity Inc. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/tversity/appflinger-go"
)

const (
	// delayBetweenKeys is used when simulating user navigation
	delayBetweenKeys = 500 * time.Millisecond

	// delayToView is used when simulating user pause to view the content it navigated to
	delayToView = 2 * time.Second
)

// Initialized from command line arguments
var serverPort string
var serverIP string
var browserURL string

var serverProtocolHost string // server IP : server port
var sessionCtx *appflinger.SessionContext

func init() {
	flag.StringVar(&serverPort, "port", "8080", "The server port")
	flag.StringVar(&serverIP, "ip", "localhost", "The server IP")
	flag.StringVar(&browserURL, "url", "https://www.youtube.com/tv?env_mediaSourceDevelopment=1", "The web address of the page to be loaded")
}

func initVars() {
	if serverPort == "80" {
		serverProtocolHost = "http://" + serverIP
	} else if serverPort == "443" {
		serverProtocolHost = "https://" + serverIP
	} else {
		serverProtocolHost = "http://" + serverIP + ":" + serverPort
	}
}

func StartSession() {
	var err error
	stub := NewAppflingerListenerStub()
	sessionCtx, err = appflinger.SessionStart(serverProtocolHost, "", browserURL, true, true, "", "", stub)
	if err != nil {
		log.Fatal("Failed to start session: ", err)
	}

	err = appflinger.SessionUIStreamStart(sessionCtx, appflinger.UI_FMT_TS_H264, false, 1000000)
	if err != nil {
		log.Fatal("Failed to start ui streaming: ", err)
	}
}

func StopSession() {
	err := appflinger.SessionUIStreamStop(sessionCtx)
	if err != nil {
		log.Fatal("Failed to stop ui sreaming: ", err)
	}

	err = appflinger.SessionStop(sessionCtx)
	if err != nil {
		log.Fatal("Failed to stop session: ", sessionCtx.SessionId, err)
	}
}

func SendEvent(code int, delay time.Duration) {
	err := appflinger.SessionSendEvent(sessionCtx, "key", code, 0, 0)
	if err != nil {
		log.Fatal("Failed to send event: ", sessionCtx.SessionId, err)
	}

	if delay != 0 {
		time.Sleep(delay)
	}
}

func RunSession(shouldStop chan bool, done chan bool) {
	StartSession()

	fmt.Println("New session:", sessionCtx.SessionId)

	// Wait till session is fully started
	select {
	case <-shouldStop:
		StopSession()
		done <- true
		return
	case <-time.After(5 * time.Second):
	}

	fmt.Println("Running session:", sessionCtx.SessionId)

	// Simulate key presses in a loop
	for {

		// Check if need to abort in a non blocking way
		select {
		case <-shouldStop:
			StopSession()
			done <- true
			return
		default:
		}

		// A sequence of navigation keys
		SendEvent(appflinger.KEY_RIGHT, delayBetweenKeys)
		SendEvent(appflinger.KEY_DOWN, delayBetweenKeys)
		SendEvent(appflinger.KEY_UP, delayBetweenKeys)
		SendEvent(appflinger.KEY_LEFT, delayBetweenKeys)

		// Some delay representing a user reading/looking before continuing the interaction
		time.Sleep(delayToView)
	}

	fmt.Println("Stopping session:", sessionCtx.SessionId)
	StopSession()
	done <- true
	return
}

func main() {
	// Handle command line arguments
	flag.Parse()
	initVars()

	shouldStop := make(chan bool, 1)
	done := make(chan bool, 1)

	// Run a session until interupted
	go RunSession(shouldStop, done)

	// Wait for Ctrl-C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	// Cleanup and exit
	fmt.Println("Exiting...")
	close(shouldStop)
	<-done
	fmt.Println("Done")
}
