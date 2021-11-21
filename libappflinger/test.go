// Copyright 2015 TVersity Inc. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	"C"
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
var sessionCtxIndex C.int

var UI_VIDEO_WIDTH C.int = 1280
var UI_VIDEO_HEIGHT C.int = 720

func init() {
	flag.StringVar(&serverPort, "port", "8080", "The server port")
	flag.StringVar(&serverIP, "ip", "localhost", "The server IP")
	flag.StringVar(&browserURL, "url", 
		"https://www.youtube.com/tv?env_mediaSourceDevelopment=1", 
		// "https://www.w3.org/2010/05/video/mediaevents.html",
		"The web address of the page to be loaded")
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

func start() {
	sessionCtxIndex = SessionStart(C.CString(serverProtocolHost), C.CString("appf"), C.CString(browserURL), 1, 1, nil, nil, UI_VIDEO_WIDTH, UI_VIDEO_HEIGHT, nil)
    if sessionCtxIndex < 0 {
        log.Println("Failed to start a session")
        return
    }

	uiFormat := appflinger.UI_FMT_JPEG_ALPHA_PNG
	// uiFormat := appflinger.UI_FMT_TS_H264
	rc := SessionUIStreamStart(sessionCtxIndex, C.CString(uiFormat), 0, 1000)
    if (rc != 0) {
		log.Println("Failed to start ui stream")
        return
    }
}

func stop() {
	rc := SessionStop(sessionCtxIndex)
	if (rc != 0) {
		log.Println("Failed to stop session")
        return
    }
}

func RunSession(shouldStop chan bool, done chan bool) {
	start()

	sessionId := SessionGetSessionId(sessionCtxIndex)

	fmt.Println("New session:", sessionId)

	// Wait till session is fully started
	select {
	case <-shouldStop:
		stop()
		done <- true
		return
	case <-time.After(5 * time.Second):
	}

	fmt.Println("Running session:", sessionId)

	for {

		// Check if need to abort in a non blocking way
		select {
		case <-shouldStop:
			fmt.Println("Stopping session:", sessionId)
			stop()
			done <- true
			return
		default:
		}

		// Some delay representing a user reading/looking before continuing the interaction
		time.Sleep(delayToView)
	}
}

func test() {
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
	shouldStop<-true

	// Cleanup and exit
	fmt.Println("Exiting...")
	close(shouldStop)
	<-done
	fmt.Println("Done")
}
