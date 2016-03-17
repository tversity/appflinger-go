// Copyright 2015 TVersity Inc. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

// This package implements the AppFlinger client SDK.
//
// It supports the following:
//  - Start/stop a session
//  - Inject input to a session
//  - Control channel implementation using HTTP long polling
//
// The client needs to implement the AppFlinger interface in order to process the control channel commands.
// An example is available under examples/stub.go which is just a stub implementation of the AppFlinger interface.
// This stub is used by examples/main.go, which illustrates how to use the client SDK. It starts a session,
// connects to its control channel and injects input in a loop until interrupted by the user, at which point
// the session is stopped.
//
// The full code for the SDK along with the examples is available under: https://github.com/tversity/appflinger-go.
package appflinger

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"golang.org/x/net/publicsuffix"
	"net/http"
	"net/http/cookiejar"
	"crypto/tls"
	"net/url"
	"strconv"
	"strings"
	"fmt"
)

const (
	// Network state constants (used by the getNetworkState() control channel command)
	NETWORK_STATE_EMPTY         = 0
	NETWORK_STATE_IDLE          = 1
	NETWORK_STATE_LOADING       = 2
	NETWORK_STATE_LOADED        = 3
	NETWORK_STATE_FORMAT_ERROR  = 4
	NETWORK_STATE_NETWORK_ERROR = 5
	NETWORK_STATE_DECODE_ERROR  = 6

	// Ready state constants  (used by the getReadykState() control channel command)
	READY_STATE_HAVE_NOTHING      = 0
	READY_STATE_HAVE_METADATA     = 1
	READY_STATE_HAVE_CURRENT_DATA = 2
	READY_STATE_HAVE_FUTURE_DATA  = 3
	READY_STATE_HAVE_ENOUGH_DATA  = 4

	// Server API endpoints (private constants)
	_SESSION_START_URL = "${PROTHOST}/osb/session/start?browser_url=${BURL}"
	_SESSION_STOP_URL  = "${PROTHOST}/osb/session/stop?session_id=${SID}"
	_SESSION_EVENT_URL = "${PROTHOST}/osb/session/event?session_id=${SID}&type=${TYPE}"
	_SESSION_CONTROL_URL = "${PROTHOST}/osb/session/control?session_id=${SID}"
	
	// Keyboard codes for injecting events
	KEY_UP        = 0x26
	KEY_DOWN      = 0x28
	KEY_LEFT      = 0x25
	KEY_RIGHT     = 0x27
	KEY_ENTER     = 0xd
	KEY_BACKSPACE = 0x8
	KEY_ESCAPE    = 0x1b
)

// The struct returned after successfully starting a session.
// It holds the information pertaining to the created browser session.
type StartSessionResp struct {
	SessionID string
}

// The client needs to implement this interface in order to process the control channel commands.
// An example is available under examples/stub.go. The "AppFlinger API and Client Integration Guide"
// describes the control channel operation and its various commands in detail.
type AppFlinger interface {
	Load(url string) (err error)
	CancelLoad() (err error)
	Pause() (err error)
	Play() (err error)
	Seek(time float64) (err error)
	GetPaused() (paused bool, err error)
	GetSeeking() (seeking bool, err error)
	GetDuration() (duration float64, err error)
	GetCurrentTime() (time float64, err error)
	GetNetworkState() (networkState int, err error)
	GetReadyState() (readyState int, err error)
	GetMaxTimeSeekable() (maxTimeSeekable float64, err error)
	SetRect(x uint, y uint, width uint, height uint) (err error)
	SetVisible(visible bool) (err error)
	SendMessage(message string) (result string, err error)
	OnPageLoad() (err error)
	OnAddressBarChanged(url string) (err error)
	OnTitleChanged(title string) (err error)
	OnPageClose() (err error)
}

type controlChannelRequest struct {
	SessionId string
	RequestId string
	Service   string

	// All possible fields

	URL     string  // used in load() and in onAddressBarChanged()
	Title   string  // used in onTitleChanged()
	Message string  // used in sendMessage()
	Time    string // used in seek()
	Visible string  // used in setVisible()

	// used in setRect
	X      string
	Y      string
	Width  string
	Height string
}

func replaceVars(str string, vars []string, vals []string) (result string) {
	result = str
	for i, _ := range vars {
		result = strings.Replace(result, vars[i], vals[i], -1)
	}
	return
}

func marshalResponse(result map[string]interface{}, respErr error) (resp string, err error) {
	if respErr == nil {
		result["result"] = "OK"
		if result["message"] == nil {
			result["message"] = ""	
		}
	} else {
		result["result"] = "ERROR"
		result["message"] = respErr.Error()
	}

	var r []byte
	r, err = json.Marshal(result)
	if err != nil {
		log.Println("Failed to create JSON for: ", result)
	} else {
		resp = string(r)
	}
	return
}

func processRPCRequest(req *controlChannelRequest, appf AppFlinger) (resp string, err error) {
	result := make(map[string]interface{})
	result["requestId"] = req.RequestId

	if req.Service == "load" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.URL))
		if appf != nil {
			err = appf.Load(req.URL)
		}
	} else if req.Service == "cancelLoad" {
		//log.Println("service: " + req.Service)
		if appf != nil {
			err = appf.CancelLoad()
		}
	} else if req.Service == "play" {
		//log.Println("service: " + req.Service)
		if appf != nil {
			err = appf.Play()
		}
	} else if req.Service == "pause" {
		//log.Println("service: " + req.Service)
		if appf != nil {
			err = appf.Pause()
		}
	} else if req.Service == "seek" {
		//log.Println(fmt.Sprintf("service: %s -- %f", req.Service, req.Time))
		var time float64
		time, err = strconv.ParseFloat(req.Time, 64)
		if err != nil {
			err = errors.New("Failed to parse float: " + req.Time)
			log.Println(err)
			resp, err = marshalResponse(result, err)
			return
		}
		if appf != nil {
			err = appf.Seek(time)
		}
	} else if req.Service == "getPaused" {
		//log.Println("service: " + req.Service)
		paused := false
		if appf != nil {
			paused, err = appf.GetPaused()
		}
		if err == nil {
			if paused {
				result["paused"] = "1"
			} else {
				result["paused"] = "0"
			}
		}
	} else if req.Service == "getSeeking" {
		//log.Println("service: " + req.Service)
		seeking := false
		if appf != nil {
			seeking, err = appf.GetSeeking()
		}
		if err == nil {
			if seeking {
				result["seeking"] = "1"
			} else {
				result["seeking"] = "0"
			}
		}
	} else if req.Service == "getDuration" {
		//log.Println("service: " + req.Service)
		duration := float64(0)
		if appf != nil {
			duration, err = appf.GetDuration()
		}
		if err == nil {
			result["duration"] = strconv.FormatFloat(duration, 'f', -1, 64)
		}
	} else if req.Service == "getCurrentTime" {
		//log.Println("service: " + req.Service)
		time := float64(0)
		if appf != nil {
			time, err = appf.GetCurrentTime()
		}
		if err == nil {
			result["currentTime"] = strconv.FormatFloat(time, 'f', -1, 64)
		}
	} else if req.Service == "getMaxTimeSeekable" {
		//log.Println("service: " + req.Service)
		time := float64(0)
		if appf != nil {
			time, err = appf.GetMaxTimeSeekable()
		}
		if err == nil {
			result["maxTimeSeekable"] = strconv.FormatFloat(time, 'f', -1, 64)
		}

	} else if req.Service == "getNetworkState" {
		//log.Println("service: " + req.Service)
		state := NETWORK_STATE_LOADED
		if appf != nil {
			state, err = appf.GetNetworkState()
		}
		if err == nil {
			result["networkState"] = strconv.Itoa(state)
		}
	} else if req.Service == "getReadyState" {
		//log.Println("service: " + req.Service)
		state := READY_STATE_HAVE_ENOUGH_DATA
		if appf != nil {
			state, err = appf.GetReadyState()
		}
		if err == nil {
			result["readyState"] = strconv.Itoa(state)
		}
	} else if req.Service == "setRect" {
		//log.Println(fmt.Sprintf("service: %s -- %s, %s, %s, %s", req.Service, req.X, req.Y, req.Width, req.Height))
		var x, y, width, height uint64
		x, err = strconv.ParseUint(req.X, 10, 0)
		if err != nil {
			err = errors.New("Failed to parse integer: " + req.X)
			log.Println(err)
			resp, err = marshalResponse(result, err)
			return
		}
		y, err = strconv.ParseUint(req.Y, 10, 0)
		if err != nil {
			err = errors.New("Failed to parse integer: " + req.Y)
			log.Println(err)
			resp, err = marshalResponse(result, err)
			return
		}
		width, err = strconv.ParseUint(req.Width, 10, 0)
		if err != nil {
			err = errors.New("Failed to parse integer: " + req.Width)
			log.Println(err)
			resp, err = marshalResponse(result, err)
			return
		}
		height, err = strconv.ParseUint(req.Height, 10, 0)
		if err != nil {
			err = errors.New("Failed to parse integer: " + req.Height)
			log.Println(err)
			resp, err = marshalResponse(result, err)
			return
		}

		if appf != nil {
			err = appf.SetRect(uint(x), uint(y), uint(width), uint(height))
		}
	} else if req.Service == "setVisible" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.Visible))
		if appf != nil {
			err = appf.SetVisible(req.Visible == "true" || req.Visible == "yes" || req.Visible == "1")
		}
	} else if req.Service == "sendMessage" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.Message))
		message := ""
		if appf != nil {
			message, err = appf.SendMessage(req.Message)
		}
		if err == nil {
			result["message"] = message
		}
	} else if req.Service == "onPageLoad" {
		//log.Println("service: ", req.Service)
		if appf != nil {
			err = appf.OnPageLoad()
		}
	} else if req.Service == "onAddressBarChanged" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.URL))
		if appf != nil {
			err = appf.OnAddressBarChanged(req.URL)
		}
	} else if req.Service == "onTitleChanged" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.Title))
		if appf != nil {
			err = appf.OnTitleChanged(req.Title)
		}
	} else if req.Service == "onPageClose" {
		//log.Println("service: ", req.Service)
		if appf != nil {
			err = appf.OnPageClose()
		}
	} else {
		err = errors.New("Unknown service: " + req.Service)
		log.Println(err)
		resp, err = marshalResponse(result, err)
		return
	}

	resp, err = marshalResponse(result, err)
	return
}

// This function is intended to be executed as a go routine.
// It connects to the control channel of the given session using HTTP long polling and remains
// connected until either stopped via the shouldStop channel or an error occurs.
// The caller needs to implement the AppFlinger interface and pass it as an argument to this function.
func ControlChannelRoutine(cookieJar *cookiejar.Jar, serverProtocolHost string, sessionId string, appf AppFlinger, shouldStop <-chan bool) (err error) {
	shouldReset := true
	postMessage := ""

	// Construct the URL
	uri := _SESSION_CONTROL_URL
	vars := []string{
		"${PROTHOST}",
		"${SID}",
	}
	vals := []string{
		serverProtocolHost,
		sessionId,
	}
	uri = replaceVars(uri, vars, vals)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	defer tr.CloseIdleConnections()

	var client http.Client
	if cookieJar != nil {
		client = http.Client{Jar: cookieJar, Transport: tr}	
	} else {
		client = http.Client{Transport: tr}
	}

	errChan := make(chan error, 1)
	for {
		uri := uri
		if shouldReset {
			uri += "&reset=1"
			shouldReset = false
		}

		var httpReq *http.Request
		var httpRes *http.Response
		httpReq, err = http.NewRequest("POST", uri, strings.NewReader(postMessage))
		if err != nil {
			err = fmt.Errorf("Control channel HTTP request creation failed with error: %v", err)
			log.Println(err)
			return
		}
		
		httpReq.Header.Set("Content-Type", "text/json")			

		// Make a long polling request to the control channel in order to process RPC requests (JSON formatted):
		// - the first invocation has &reset=1 and no payload
		// - subsequent invocation do not have &reset=1 and do have a payload which is the response to the
		//   previously received RPC request
		// The request is made in a go function so that we can cancel it when requested to do so

		go func() {
			httpRes, err = client.Do(httpReq)
			if err != nil {
				err = fmt.Errorf("Control channel HTTP request failed with error: %v", err)
				log.Println(err)
			}
			errChan <- err
		} ()
			
		// Wait for the http request to complete
		select {
        	case <-shouldStop:
            	tr.CancelRequest(httpReq)
				return
        	case err = <- errChan:
				if err != nil {
					return
				}
		}

		if httpRes.StatusCode != http.StatusOK {
			err = fmt.Errorf("Control channel HTTP request failed with status: %s", httpRes.Status)
			log.Println(err)
			return
		}
		if httpRes.Header.Get("Content-Type") != "text/json" {
			msg := "Invalid response content type: " + httpRes.Header.Get("Content-Type")
			log.Println(msg)
			err = errors.New(msg)
			return
		}

		// Parse the response
		req := &controlChannelRequest{}
		dec := json.NewDecoder(httpRes.Body)
		for {
			err = dec.Decode(req)
			if err == io.EOF {
				err = nil
				break
			} else if err != nil {				
				// Check if need to abort in a non blocking way		
				select {
					case <-shouldStop:
						err = nil
						return
					default:
				}

				// This is most likely a timeout
				log.Println("Failed to read and/or parse control channel HTTP request body with error: ", err)
				shouldReset = true
				break
			}
		}
		if err != nil || req.Service == "" {
			httpRes.Body.Close()
			postMessage = ""
			continue
		}

		postMessage, err = processRPCRequest(req, appf)
		if err != nil {
			log.Println("Failed to process RPC message: ", req)
			postMessage = ""
		}
		httpRes.Body.Close()
	}
	
	return
}

func printCookies(cookieJar *cookiejar.Jar, uri string) {
	u,_ := url.Parse(uri)
	for _,c := range cookieJar.Cookies(u) {
		log.Println(c)	
	}
}

func apiReq(cookieJar *cookiejar.Jar, uri string, resp interface{}) (err error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	defer tr.CloseIdleConnections()
	
	var client http.Client
	if cookieJar != nil {
		client = http.Client{Jar: cookieJar, Transport: tr}	
	} else {
		client = http.Client{Transport: tr}
	}
	
	var res *http.Response
	res, err = client.Get(uri)
	if err != nil {
		err = fmt.Errorf("HTTP request failed with error: %v, uri: %s", err, uri)
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("HTTP request failed with status: %s, uri: %s", res.Status, uri)
		return
	}

	if resp == nil {
		return
	}

	// Parse the response
	dec := json.NewDecoder(res.Body)

	for {
		err = dec.Decode(resp)
		if err == io.EOF {
			err = nil
			break
		} else if err != nil {
			err = fmt.Errorf("Failed to read and/or parse HTTP request body with error: %v, uri: %s", err, uri)
			return
		}
	}

	return
}

// This function is used to start a new session or navigate an existing one to a new address.
// The arguments to this function are as per the descirption of the /osb/session/start API in
// the "AppFlinger API and Client Integration Guide".
// The function returns a struct with the data returned from the server (namely the session id) as well as
// a Go cookie jar, which needs to be used in all subsequent API requests for this session. Note that Cookies
// are important for load balancing stickyness such that a session start request is made without any cookies
// but may return a cookie when a load balancer is used. This returned cookie must be passed in any subsequent
// requests that need to use the session so that the load balancer will hit the correct server. 
func SessionStart(serverProtocolHost string, browserURL string,  pullMode bool, isVideoPassthru bool, browserUIOutputURL string,
videoStreamURI string) (rsp *StartSessionResp, cookieJar *cookiejar.Jar, err error) {
	rsp = nil
	cookieJar = nil

	// Create the cookie jar first
	 options := cookiejar.Options{
        PublicSuffixList: publicsuffix.List,
    }
    cookieJar, err = cookiejar.New(&options)
    if err != nil {
		cookieJar = nil
        return
    }

	// Construct the URL
	uri := _SESSION_START_URL
	if !isVideoPassthru {
		uri += "&video_stream_uri=${VURL}"
	}
	if pullMode {
		uri += "&browser_ui_video_pull=yes"
	} else {
		uri += "&browser_ui_output_url=${UURL}"
	}

	uri = replaceVars(uri, []string{
		"${PROTHOST}",
		"${BURL}",
		"${VURL}",
		"${UURL}",
	}, []string{
		serverProtocolHost,
		url.QueryEscape(browserURL),
		url.QueryEscape(videoStreamURI),
		url.QueryEscape(browserUIOutputURL),
	})

	// Make the request
	rsp = &StartSessionResp{}
	err = apiReq(cookieJar, uri, rsp)
	if err != nil {
		rsp = nil
		cookieJar = nil
		return
	}
	return
}

// This function is used to stop a session.
// The arguments to this function are as per the descirption of the /osb/session/stop API in
// the "AppFlinger API and Client Integration Guide", with the exception of the cookie jar which was
// returned when the session was started.
func SessionStop(cookieJar *cookiejar.Jar, serverProtocolHost string, sessionId string) (err error) {
	// Construct the URL
	uri := replaceVars(_SESSION_STOP_URL, []string{
		"${PROTHOST}",
		"${SID}",
	}, []string{
		serverProtocolHost,
		sessionId,
	})

	// Make the request
	err = apiReq(cookieJar, uri, nil)
	return
}

// This function is used to inject input into a session.
// The arguments to this function are as per the descirption of the /osb/session/event API in
// the "AppFlinger API and Client Integration Guide", with the exception of the cookie jar which was
// returned when the session was started.
func SessionEvent(cookieJar *cookiejar.Jar, serverProtocolHost string, sessionId string, eventType string, code int, x int, y int) (err error) {
	// Construct the URL
	uri := _SESSION_EVENT_URL
	eventType = strings.ToLower(eventType)
	if eventType == "key" {
		uri += "&code=${KEYCODE}"	
	} else if eventType == "click" {
		uri += "&x=${X}&y=${Y}"
	} else {
		err = errors.New("Invalid event type: " + eventType)
		return
	}

	uri = replaceVars(uri, []string{
		"${PROTHOST}",
		"${SID}",
		"${TYPE}",
		"${KEYCODE}",
		"${X}",
		"${Y}",
	}, []string{
		serverProtocolHost,
		sessionId,
		eventType,
		strconv.Itoa(code),
		strconv.Itoa(x),
		strconv.Itoa(y),
	})

	// Make the request
	err = apiReq(cookieJar, uri, nil)
	return
}
