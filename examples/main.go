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
	"net/http/cookiejar"
	"github.com/tversity/appflinger-go"
)

const (
	DELAY_BETWEEN_KEYS = 500 * time.Millisecond
	DELAY_TO_VIEW = 2 * time.Second
)

// Initialized from command line arguments
var serverPort string
var serverIP string
var browserURL string

var serverProtocolHost string // server IP : server port
var sessionId string
var cookieJar *cookiejar.Jar

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
	rsp, cj, err := appflinger.SessionStart(serverProtocolHost, browserURL,  true, true, "", "")
	if err != nil {
		log.Fatal("Failed to start session: ", err)
	}
	cookieJar = cj
	sessionId = rsp.SessionID
}

func StopSession() {
	err := appflinger.SessionStop(cookieJar, serverProtocolHost, sessionId)
	if err != nil {
		log.Fatal("Failed to stop session: ", sessionId, err)
	}
}

func SendEvent(code int, delay time.Duration) {
	err := appflinger.SessionEvent(cookieJar, serverProtocolHost, sessionId, "key", code, 0, 0)
	if err != nil {
		log.Fatal("Failed to send event: ", sessionId, err)
	}
	
	if delay != 0 {
		time.Sleep(delay)
	}
}

func ControlChannel(shouldStop chan bool) {
	// Use a stub implementation of control channel callbacks used for testing / demonstration
	// A real client implementation will need to replace the stub with a complete implementation
	stub := NewAppFlingerStub()
	err := appflinger.ControlChannelRoutine(cookieJar, serverProtocolHost, sessionId, stub, shouldStop)
	if err != nil {
		log.Fatal("Failed to connect to control channel with error: ", err)
	}
}

func RunSession(shouldStop chan bool, done chan bool) () {
	StartSession()

	fmt.Println("New session:", sessionId)

	// Wait till session is fully started
	select {
		case <- shouldStop:
			StopSession()
			done <- true
			return
		case <-time.After(5 * time.Second):
    }

	fmt.Println("Running session:", sessionId)

	// Control channel implementation
	go ControlChannel(shouldStop)

	// Simulate key presses in a loop
	for {
			
		// Check if need to abort in a non blocking way		
		select {
			case <- shouldStop:
				StopSession()
				done <- true
				return
			default:
		}

		// A sequence of navigation keys
		SendEvent(appflinger.KEY_RIGHT, DELAY_BETWEEN_KEYS)
		SendEvent(appflinger.KEY_DOWN, DELAY_BETWEEN_KEYS)
		SendEvent(appflinger.KEY_UP, DELAY_BETWEEN_KEYS)
		SendEvent(appflinger.KEY_LEFT, DELAY_BETWEEN_KEYS)

		// Some delay representing a user reading/looking before continuing the interaction
		time.Sleep(DELAY_TO_VIEW)
	}
	
	fmt.Println("Stopping session:", sessionId)
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
	<- c
	
	// Cleanup and exit
	fmt.Println("Exiting...")
	close(shouldStop)
	<- done
	log.Println("Done")
}
