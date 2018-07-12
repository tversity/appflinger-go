// Copyright 2015 TVersity Inc. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

// This package implements the AppFlinger client SDK.
//
// It supports the following:
//  - Start/stop a session
//  - Inject input to a session
//  - Control channel implementation using HTTP long polling
//  - UI video streaming and demuxing
//
// The client needs to implement the AppFlingerListener interface in order to process the control channel commands.
// An example is available under examples/stub.go which is just a stub implementation of the AppFlingerListener interface.
// This stub is used by examples/main.go, which illustrates how to use the client SDK. It starts a session,
// and injects input in a loop until interrupted by the user, at which point the session is stopped.
//
// The full code for the SDK along with the examples is available under: https://github.com/tversity/appflinger-go.
package appflinger

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"

	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/codec/h264parser"
	"github.com/nareix/joy4/format/ts"
	"golang.org/x/net/publicsuffix"
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
	_SESSION_START_URL   = "${PROTHOST}/osb/session/start?browser_url=${BURL}"
	_SESSION_STOP_URL    = "${PROTHOST}/osb/session/stop?session_id=${SID}"
	_SESSION_EVENT_URL   = "${PROTHOST}/osb/session/event?session_id=${SID}&type=${TYPE}"
	_SESSION_CONTROL_URL = "${PROTHOST}/osb/session/control?session_id=${SID}"
	_SESSION_UI_URL      = "${PROTHOST}/osb/session/ui?session_id=${SID}&fmt=${FMT}&ts_discon=${TSDISCON}"

	// Keyboard codes for injecting events
	KEY_UP        = 0x26
	KEY_DOWN      = 0x28
	KEY_LEFT      = 0x25
	KEY_RIGHT     = 0x27
	KEY_ENTER     = 0xd
	KEY_BACKSPACE = 0x8
	KEY_ESCAPE    = 0x1b

	// Media formats supported for UI stream
	UI_FMT_TS_H264  = "mp2t;h264"
	UI_FMT_MP4_H264 = "mp4;h264"
	UI_FMT_WEBM_VP8 = "webm;vp8"
	UI_FMT_MPD_TS   = "mpd;mp2"
	UI_FMT_MPD_MP4  = "mpd;mp4"
	UI_FMT_MPD_WEBM = "mpd;webm"
	UI_FMT_JPEG     = "jpeg"
	UI_FMT_PNG      = "png"

	// Do not read more than this number of bytes as a safety mechanism against attacks, etc.
	_HTTP_MAX_RESPONSE_SIZE = 10000000
)

// Allowed formats for streaming
var _ALLOWED_UI_FMT = map[string]bool{
	UI_FMT_TS_H264:  true,
	UI_FMT_MP4_H264: true,
	UI_FMT_WEBM_VP8: true,
	UI_FMT_MPD_TS:   true,
	UI_FMT_MPD_MP4:  true,
	UI_FMT_MPD_WEBM: true,
	UI_FMT_JPEG:     true,
	UI_FMT_PNG:      true,
}

// The struct to which the JSON returned after successfully starting a session is parsed.
type sessionStartResp struct {
	SessionID string
}

// SessionContext is returned when starting a session and needs to be passed to subsequent operations on the session.
type SessionContext struct {
	SessionId                               string
	appflingerListener                      AppflingerListener
	CookieJar                               *cookiejar.Jar
	ServerProtocolHost                      string
	isUIStreaming                           bool
	shouldStopSession, shouldStopUI, isDone chan bool
}

// The struct to which the JSON received in a control channel as a request, is parsed.
// We put here union of all possible fields and the JSON decoder will populate the relevant
// on the type of the request
type controlChannelRequest struct {
	// Basic fields that every control channel request has

	SessionId   string
	RequestId   string
	InstanceId  string
	Service     string
	PayloadSize string // used when payload exists (e.g. loadResource() and appendBuffer())

	// All other possible fields

	URL     string // used in load() and in onAddressBarChanged()
	Title   string // used in onTitleChanged()
	Message string // used in sendMessage()
	Time    string // used in seek()
	Visible string // used in setVisible()

	// Used in addSourceBuffer() and various other MSE related functions
	SourceId string
	Type     string

	// Used in appendBuffer()
	AppendWindowStart string
	AppendWindowEnd   string
	BufferId          string
	BufferOffset      string
	BufferLength      string

	// Used in loadResource()
	ResourceId     string
	Url            string
	Method         string
	Headers        string
	ByteRange      string
	SequenceNumber string

	// Used in setRect
	X      string
	Y      string
	Width  string
	Height string
}

// LoadResourceResult is the returned result by LoadResource() in the AppflingerListener interface
type LoadResourceResult struct {
	Code         string
	Headers      string
	BufferId     string
	BufferLength int
	Payload      []byte
}

// GetBufferedResult is the returned result by GetBuffered() in the AppflingerListener interface
type GetBufferedResult struct {
	Start []float64
	End   []float64
}

// Added this set of methods since gomobile cannot generate bindings to arrays

func (result *GetBufferedResult) GetLength() int {
	if len(result.Start) != len(result.End) {
		log.Println("Internal error, array length mismatch")
	}
	return len(result.Start)
}
func (result *GetBufferedResult) SetLength(length int) {
	result.Start = make([]float64, length)
	result.End = make([]float64, length)
}
func (result *GetBufferedResult) GetStart(index int) float64 {
	return result.Start[index]
}
func (result *GetBufferedResult) SetStart(index int, value float64) {
	result.Start[index] = value
}
func (result *GetBufferedResult) GetEnd(index int) float64 {
	return result.End[index]
}
func (result *GetBufferedResult) SetEnd(index int, value float64) {
	result.End[index] = value
}

// AppflingerListener is the interface a client needs to implement in order to process the control channel commands.
// An example is available under examples/stub.go. The "AppFlinger API and Client Integration Guide"
// describes the control channel operation and its various commands in detail.
type AppflingerListener interface {
	Load(instanceId string, url string) (err error)
	CancelLoad(instanceId string) (err error)
	Pause(instanceId string) (err error)
	Play(instanceId string) (err error)
	Seek(instanceId string, time float64) (err error)
	GetPaused(instanceId string) (paused bool, err error)
	GetSeeking(instanceId string) (seeking bool, err error)
	GetDuration(instanceId string) (duration float64, err error)
	GetCurrentTime(instanceId string) (time float64, err error)
	GetNetworkState(instanceId string) (networkState int, err error)
	GetReadyState(instanceId string) (readyState int, err error)
	GetMaxTimeSeekable(instanceId string) (maxTimeSeekable float64, err error)
	GetBuffered(instanceId string, result *GetBufferedResult) (err error)
	SetRect(instanceId string, x int, y int, width int, height int) (err error)
	SetVisible(instanceId string, visible bool) (err error)
	AddSourceBuffer(instanceId string, sourceId string, mimeType string) (err error)
	RemoveSourceBuffer(instanceId string, sourceId string) (err error)
	ResetSourceBuffer(instanceId string, sourceId string) (err error)
	AppendBuffer(instanceId string, sourceId string, appendWindowStart float64, appendWindowEnd float64, bufferId string, bufferOffset int,
		bufferLength int, payload []byte, result *GetBufferedResult) (err error)
	LoadResource(url string, method string, headers string, resourceId string, byteRangeStart int, byteRangeEnd int,
		sequenceNumber int, payload []byte, result *LoadResourceResult) (err error)
	SendMessage(message string) (result string, err error)
	OnPageLoad() (err error)
	OnAddressBarChanged(url string) (err error)
	OnTitleChanged(title string) (err error)
	OnPageClose() (err error)
	OnUIFrame(isCodecConfig bool, isKeyFrame bool, idx int, pts int, dts int, data []byte) (err error)
}

func replaceVars(str string, vars []string, vals []string) (result string) {
	result = str
	for i := range vars {
		result = strings.Replace(result, vars[i], vals[i], -1)
	}
	return
}

func marshalRPCResponse(result map[string]interface{}, resultPayload []byte, respErr error) (resp []byte, err error) {
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
		resp = append(r, resultPayload...)
	}
	return
}

func processRPCRequest(req *controlChannelRequest, payload []byte, appf AppflingerListener) (resp []byte, err error) {
	result := make(map[string]interface{})
	result["requestId"] = req.RequestId
	var resultPayload []byte = nil

	if req.Service == "load" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.URL))
		if appf != nil {
			err = appf.Load(req.InstanceId, req.URL)
		}
	} else if req.Service == "cancelLoad" {
		//log.Println("service: " + req.Service)
		if appf != nil {
			err = appf.CancelLoad(req.InstanceId)
		}
	} else if req.Service == "play" {
		//log.Println("service: " + req.Service)
		if appf != nil {
			err = appf.Play(req.InstanceId)
		}
	} else if req.Service == "pause" {
		//log.Println("service: " + req.Service)
		if appf != nil {
			err = appf.Pause(req.InstanceId)
		}
	} else if req.Service == "seek" {
		//log.Println(fmt.Sprintf("service: %s -- %f", req.Service, req.Time))
		var time float64
		time, err = strconv.ParseFloat(req.Time, 64)
		if err != nil {
			err = errors.New("Failed to parse float: " + req.Time)
			log.Println(err)
			resp, err = marshalRPCResponse(result, resultPayload, err)
			return
		}
		if appf != nil {
			err = appf.Seek(req.InstanceId, time)
		}
	} else if req.Service == "getPaused" {
		//log.Println("service: " + req.Service)
		paused := false
		if appf != nil {
			paused, err = appf.GetPaused(req.InstanceId)
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
			seeking, err = appf.GetSeeking(req.InstanceId)
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
			duration, err = appf.GetDuration(req.InstanceId)
		}
		if err == nil {
			result["duration"] = strconv.FormatFloat(duration, 'f', -1, 64)
		}
	} else if req.Service == "getCurrentTime" {
		//log.Println("service: " + req.Service)
		time := float64(0)
		if appf != nil {
			time, err = appf.GetCurrentTime(req.InstanceId)
		}
		if err == nil {
			result["currentTime"] = strconv.FormatFloat(time, 'f', -1, 64)
		}
	} else if req.Service == "getMaxTimeSeekable" {
		//log.Println("service: " + req.Service)
		time := float64(0)
		if appf != nil {
			time, err = appf.GetMaxTimeSeekable(req.InstanceId)
		}
		if err == nil {
			result["maxTimeSeekable"] = strconv.FormatFloat(time, 'f', -1, 64)
		}
	} else if req.Service == "getNetworkState" {
		//log.Println("service: " + req.Service)
		state := NETWORK_STATE_LOADED
		if appf != nil {
			state, err = appf.GetNetworkState(req.InstanceId)
		}
		if err == nil {
			result["networkState"] = strconv.Itoa(state)
		}
	} else if req.Service == "getReadyState" {
		//log.Println("service: " + req.Service)
		state := READY_STATE_HAVE_ENOUGH_DATA
		if appf != nil {
			state, err = appf.GetReadyState(req.InstanceId)
		}
		if err == nil {
			result["readyState"] = strconv.Itoa(state)
		}
	} else if req.Service == "getBuffered" {
		//log.Println("service: " + req.Service)
		if appf != nil {
			// Time range of buffered portions, there can be gaps that are unbuffered hence
			// we are dealing with two arrays and not two scalars.
			var getBufferedResult GetBufferedResult
			err = appf.GetBuffered(req.InstanceId, &getBufferedResult)
			if err == nil {
				if getBufferedResult.Start != nil && getBufferedResult.End != nil {
					result["start"] = getBufferedResult.Start
					result["end"] = getBufferedResult.End
				}
			}
		}
	} else if req.Service == "setRect" {
		//log.Println(fmt.Sprintf("service: %s -- %s, %s, %s, %s", req.Service, req.X, req.Y, req.Width, req.Height))
		var x, y, width, height uint64
		x, err = strconv.ParseUint(req.X, 10, 0)
		if err != nil {
			err = errors.New("Failed to parse integer: " + req.X)
			log.Println(err)
			resp, err = marshalRPCResponse(result, resultPayload, err)
			return
		}
		y, err = strconv.ParseUint(req.Y, 10, 0)
		if err != nil {
			err = errors.New("Failed to parse integer: " + req.Y)
			log.Println(err)
			resp, err = marshalRPCResponse(result, resultPayload, err)
			return
		}
		width, err = strconv.ParseUint(req.Width, 10, 0)
		if err != nil {
			err = errors.New("Failed to parse integer: " + req.Width)
			log.Println(err)
			resp, err = marshalRPCResponse(result, resultPayload, err)
			return
		}
		height, err = strconv.ParseUint(req.Height, 10, 0)
		if err != nil {
			err = errors.New("Failed to parse integer: " + req.Height)
			log.Println(err)
			resp, err = marshalRPCResponse(result, resultPayload, err)
			return
		}

		if appf != nil {
			err = appf.SetRect(req.InstanceId, int(x), int(y), int(width), int(height))
		}
	} else if req.Service == "setVisible" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.Visible))
		if appf != nil {
			err = appf.SetVisible(req.InstanceId, req.Visible == "true" || req.Visible == "yes" || req.Visible == "1")
		}
	} else if req.Service == "addSourceBuffer" {
		//log.Println(fmt.Sprintf("service: %s -- %s, %s", req.Service, req.SourceId, req.Type))
		if appf != nil {
			err = appf.AddSourceBuffer(req.InstanceId, req.SourceId, req.Type)
		}
	} else if req.Service == "removeSourceBuffer" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.SourceId))
		if appf != nil {
			err = appf.RemoveSourceBuffer(req.InstanceId, req.SourceId)
		}
	} else if req.Service == "resetSourceBuffer" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.SourceId))
		if appf != nil {
			err = appf.ResetSourceBuffer(req.InstanceId, req.SourceId)
		}
	} else if req.Service == "appendBuffer" {
		/*log.Println(fmt.Sprintf("service: %s -- %s, %s, %s, %s, %s, %s, %s", req.Service, req.SourceId,
		req.AppendWindowStart, req.AppendWindowEnd, req.BufferId, req.BufferOffset, req.BufferLength))
		*/
		var appendWindowStart, appendWindowEnd float64
		if req.AppendWindowStart == "inf" {
			appendWindowStart = math.Inf(1)
		} else {
			appendWindowStart, err = strconv.ParseFloat(req.AppendWindowStart, 64)
			if err != nil {
				err = errors.New("Failed to parse float: " + req.AppendWindowStart)
				log.Println(err)
				resp, err = marshalRPCResponse(result, resultPayload, err)
				return
			}
		}
		if req.AppendWindowEnd == "inf" {
			appendWindowEnd = math.Inf(1)
		} else {
			appendWindowEnd, err = strconv.ParseFloat(req.AppendWindowEnd, 64)
			if err != nil {
				err = errors.New("Failed to parse float: " + req.AppendWindowEnd)
				log.Println(err)
				resp, err = marshalRPCResponse(result, resultPayload, err)
				return
			}
		}

		var bufferOffset, bufferLength uint64
		if req.BufferId != "" {
			bufferOffset, err = strconv.ParseUint(req.BufferOffset, 10, 0)
			if err != nil {
				err = errors.New("Failed to parse integer: " + req.BufferOffset)
				log.Println(err)
				resp, err = marshalRPCResponse(result, resultPayload, err)
				return
			}
			bufferLength, err = strconv.ParseUint(req.BufferLength, 10, 0)
			if err != nil {
				err = errors.New("Failed to parse integer: " + req.BufferLength)
				log.Println(err)
				resp, err = marshalRPCResponse(result, resultPayload, err)
				return
			}
		}

		if appf != nil {
			var getBufferedResult GetBufferedResult
			err = appf.AppendBuffer(req.InstanceId, req.SourceId, appendWindowStart, appendWindowEnd, req.BufferId,
				int(bufferOffset), int(bufferLength), payload, &getBufferedResult)
			if err == nil {
				if getBufferedResult.Start != nil && getBufferedResult.End != nil {
					result["start"] = getBufferedResult.Start
					result["end"] = getBufferedResult.End
				}
			}
		}
	} else if req.Service == "loadResource" {
		/*log.Println(fmt.Sprintf("service: %s -- %s, %s, %s, %s", req.Service, req.Url, req.Method, req.Headers,
		req.ResourceId, req.ByteRange, req.SequenceNumber))
		*/

		var sequenceNumber uint64
		byteRange := make([]uint64, 2)
		if req.ResourceId != "" {
			byteRangeArray := strings.Split(req.ByteRange, "-")
			if len(byteRangeArray) != 2 {
				err = errors.New("Failed to parse range: " + req.ByteRange)
				log.Println(err)
				resp, err = marshalRPCResponse(result, resultPayload, err)
				return
			}

			byteRange[0], err = strconv.ParseUint(byteRangeArray[0], 10, 0)
			if err != nil {
				err = errors.New("Failed to parse integer: " + byteRangeArray[0])
				log.Println(err)
				resp, err = marshalRPCResponse(result, resultPayload, err)
				return
			}
			byteRange[1], err = strconv.ParseUint(byteRangeArray[1], 10, 0)
			if err != nil {
				err = errors.New("Failed to parse integer: " + byteRangeArray[1])
				log.Println(err)
				resp, err = marshalRPCResponse(result, resultPayload, err)
				return
			}

			sequenceNumber, err = strconv.ParseUint(req.SequenceNumber, 10, 0)
			if err != nil {
				err = errors.New("Failed to parse integer: " + req.SequenceNumber)
				log.Println(err)
				resp, err = marshalRPCResponse(result, resultPayload, err)
				return
			}
		}
		if appf != nil {
			var loadResourceResult LoadResourceResult
			err = appf.LoadResource(req.Url, req.Method, req.Headers, req.ResourceId,
				int(byteRange[0]), int(byteRange[1]), int(sequenceNumber), payload, &loadResourceResult)
			if err == nil {
				result["code"] = loadResourceResult.Code
				result["headers"] = loadResourceResult.Headers
				if req.ResourceId != "" {
					result["bufferId"] = loadResourceResult.BufferId
					result["bufferLength"] = strconv.Itoa(loadResourceResult.BufferLength)
				}
			}
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
		resp, err = marshalRPCResponse(result, resultPayload, err)
		return
	}

	resp, err = marshalRPCResponse(result, resultPayload, err)
	return
}

// controlChannelRun is intended to be executed as a go routine.
// It connects to the control channel of the given session using HTTP long polling and remains
// connected until either stopped via the shouldStop channel or an error occurs.
// The caller needs to implement the AppFlinger interface and pass it as an argument to this function.
func controlChannelRun(ctx *SessionContext, appf AppflingerListener) (err error) {
	shouldReset := true
	var postMessage []byte = nil

	// Construct the URL
	uri := _SESSION_CONTROL_URL
	vars := []string{
		"${PROTHOST}",
		"${SID}",
	}
	vals := []string{
		ctx.ServerProtocolHost,
		ctx.SessionId,
	}
	uri = replaceVars(uri, vars, vals)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	defer tr.CloseIdleConnections()

	var client http.Client
	if ctx.CookieJar != nil {
		client = http.Client{Jar: ctx.CookieJar, Transport: tr}
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
		httpReq, err = http.NewRequest("POST", uri, bytes.NewReader(postMessage))
		if err != nil {
			err = fmt.Errorf("Control channel HTTP request creation failed with error: %v", err)
			log.Println(err)
			ctx.isDone <- true
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
		}()

		// Wait for the http request to complete
		select {
		case <-ctx.shouldStopSession:
			tr.CancelRequest(httpReq)
			ctx.isDone <- true
			return
		case err = <-errChan:
			if err != nil {
				ctx.isDone <- true
				return
			}
		}

		if httpRes.StatusCode != http.StatusOK {
			err = fmt.Errorf("Control channel HTTP request failed with status: %s", httpRes.Status)
			log.Println(err)
			httpRes.Body.Close()
			ctx.isDone <- true
			return
		}
		if httpRes.Header.Get("Content-Type") != "text/json" {
			msg := "Invalid response content type: " + httpRes.Header.Get("Content-Type")
			log.Println(msg)
			err = errors.New(msg)
			httpRes.Body.Close()
			ctx.isDone <- true
			return
		}

		var body []byte
		// Invoke the decode function in a go routine since it can block
		go func() {
			body, err = ioutil.ReadAll(io.LimitReader(httpRes.Body, _HTTP_MAX_RESPONSE_SIZE))
			errChan <- err
		}()
		// Wait for the http response reading to complete
		select {
		case <-ctx.shouldStopSession:
			httpRes.Body.Close()
			ctx.isDone <- true
			return
		case err = <-errChan:
			break
		}
		if err != nil {
			err = fmt.Errorf("Failed to read response from control channel with error: %v", err)
			log.Println(err)
			httpRes.Body.Close()
			ctx.isDone <- true
			return
		}

		// Look for \n\n
		jsonEndPos := bytes.Index(body, []byte("\n\n"))
		if jsonEndPos < 0 {
			err = fmt.Errorf("Invalid response from control channel, missing end of message newlines")
			log.Println(err)
			httpRes.Body.Close()
			ctx.isDone <- true
			return
		}
		jsonEndPos += 2

		// Parse the response
		req := &controlChannelRequest{}
		err = json.Unmarshal(body[:jsonEndPos], req)
		if err != nil {
			// Check if need to abort in a non blocking way
			select {
			case <-ctx.shouldStopSession:
				err = nil
				httpRes.Body.Close()
				ctx.isDone <- true
				return
			default:
			}

			// This is most likely a timeout
			log.Println("Failed to parse control channel HTTP response body with error: ", err)
			shouldReset = true
			postMessage = nil
			continue
		}

		// Empty messages are sent periodically to keep the connection open
		if req.Service == "" {
			httpRes.Body.Close()
			postMessage = nil
			continue
		}

		payload := body[jsonEndPos:]
		if req.PayloadSize != "" || len(payload) > 0 {
			var payloadSize uint64
			payloadSize, err = strconv.ParseUint(req.PayloadSize, 10, 0)
			if err != nil {
				err = errors.New("Failed to parse payload size integer: " + req.PayloadSize)
			}
			if uint64(len(payload)) != payloadSize {
				err = fmt.Errorf("Payload size mismatch, %d != %d", len(payload), payloadSize)
				log.Println(err)
			}
			if err != nil {
				log.Println(err)
				httpRes.Body.Close()
				postMessage = nil
				continue
			}
		}

		postMessage, err = processRPCRequest(req, payload, appf)
		if err != nil {
			log.Println("Failed to process RPC message: ", req)
			postMessage = nil
		}
		httpRes.Body.Close()
	}
}

func printCookies(cookieJar *cookiejar.Jar, uri string) {
	u, _ := url.Parse(uri)
	for _, c := range cookieJar.Cookies(u) {
		log.Println(c)
	}
}

func httpGet(cookieJar *cookiejar.Jar, uri string, shouldStop chan bool) (reader io.ReadCloser, err error) {
	reader = nil

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

	var httpReq *http.Request
	var httpRes *http.Response
	httpReq, err = http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}

	if shouldStop != nil {
		errChan := make(chan error, 1)
		go func() {
			httpRes, err = client.Do(httpReq)
			if err != nil {
				err = fmt.Errorf("HTTP request failed with error: %v, uri: %s", err, uri)
			}
			errChan <- err
		}()

		// Wait for the http request to complete
		select {
		case <-shouldStop:
			tr.CancelRequest(httpReq)
			return
		case err = <-errChan:
			if err != nil {
				return
			}
		}
	} else {
		httpRes, err = client.Do(httpReq)
		if err != nil {
			err = fmt.Errorf("HTTP request failed with error: %v, uri: %s", err, uri)
			return
		}
	}

	if httpRes.StatusCode != http.StatusOK {
		err = fmt.Errorf("HTTP request failed with status: %s, uri: %s", httpRes.Status, uri)
		httpRes.Body.Close()
		return
	}

	return httpRes.Body, nil
}

func apiReq(cookieJar *cookiejar.Jar, uri string, shouldStop chan bool, resp interface{}) (err error) {
	reader, e := httpGet(cookieJar, uri, shouldStop)
	if e != nil {
		return e
	}
	defer reader.Close()

	if resp == nil {
		return
	}

	// Parse the response
	dec := json.NewDecoder(reader)

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

func controlChannelRoutine(ctx *SessionContext, appf AppflingerListener) {
	err := controlChannelRun(ctx, appf)
	if err != nil {
		log.Println("Failed to connect to control channel with error: ", err)
	}
}

// SessionStart is used to start a new session or navigate an existing one to a new address.
// The arguments to this function are as per the descirption of the /osb/session/start API in
// the "AppFlinger API and Client Integration Guide".
func SessionStart(serverProtocolHost string, sessionId string, browserURL string, pullMode bool, isVideoPassthru bool, browserUIOutputURL string,
	videoStreamURL string, appf AppflingerListener) (ctx *SessionContext, err error) {
	var cookieJar *cookiejar.Jar
	ctx = nil

	// Create the cookie jar first, which needs to be used in all API requests for this session. Note that Cookies
	// are important for load balancing stickyness such that a session start request is made without any cookies
	// but may return a cookie when a load balancer is used. This returned cookie must be passed in any subsequent
	// requests that need to use the session so that the load balancer will hit the correct server.
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
	if sessionId != "" {
		uri += "&session_id=${SID}"
	}

	uri = replaceVars(uri, []string{
		"${PROTHOST}",
		"${BURL}",
		"${VURL}",
		"${UURL}",
		"${SID}",
	}, []string{
		serverProtocolHost,
		url.QueryEscape(browserURL),
		url.QueryEscape(videoStreamURL),
		url.QueryEscape(browserUIOutputURL),
		url.QueryEscape(sessionId),
	})

	// Make the request
	// We get here a struct with the data returned from the server (namely the session id)
	resp := &sessionStartResp{}
	err = apiReq(cookieJar, uri, nil, resp)
	if err != nil {
		log.Println("Failed to start session: ", err)
		resp = nil
		cookieJar = nil
		return
	}
	ctx = &SessionContext{}
	ctx.ServerProtocolHost = serverProtocolHost
	ctx.SessionId = resp.SessionID
	ctx.appflingerListener = appf
	ctx.CookieJar = cookieJar
	ctx.shouldStopSession = make(chan bool, 1)
	ctx.shouldStopUI = make(chan bool, 1)
	ctx.isDone = make(chan bool, 1)
	go controlChannelRoutine(ctx, appf)
	return
}

// SessionStop is used to stop a session.
func SessionStop(ctx *SessionContext) (err error) {

	// Stop and Wait for ui streaming to complete
	if ctx.isUIStreaming {
		SessionUIStreamStop(ctx)
	}

	// Stop the control channel go routine and the ui steaming go routine
	close(ctx.shouldStopSession)

	// Wait for control channel to confirm
	<-ctx.isDone

	// Construct the URL
	uri := replaceVars(_SESSION_STOP_URL, []string{
		"${PROTHOST}",
		"${SID}",
	}, []string{
		ctx.ServerProtocolHost,
		url.QueryEscape(ctx.SessionId),
	})

	// Make the request
	err = apiReq(ctx.CookieJar, uri, nil, nil)
	if err != nil {
		log.Println("Failed to stop session: ", err)
		return
	}
	return
}

// SessionSendEvent is used to inject input into a session.
func SessionSendEvent(ctx *SessionContext, eventType string, code int, x int, y int) (err error) {
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
		ctx.ServerProtocolHost,
		url.QueryEscape(ctx.SessionId),
		eventType,
		strconv.Itoa(code),
		strconv.Itoa(x),
		strconv.Itoa(y),
	})

	// Make the request
	err = apiReq(ctx.CookieJar, uri, ctx.shouldStopSession, nil)
	return
}

// SessionGetUIURL is used to obtain the HTTP URL from which the browser UI can be streamed.
// Note that one should also consider the cookies for that URL before making the HTTP request.
func SessionGetUIURL(ctx *SessionContext, fmt string, tsDiscon bool, bitrate int) (uri string, err error) {
	err = nil

	// Construct the URL
	uri = _SESSION_UI_URL
	if bitrate > 0 {
		uri += "&bitrate=${BITRATE}"
	}

	if !_ALLOWED_UI_FMT[fmt] {
		err = errors.New("Invalid format: " + fmt)
		return
	}
	tsDisconStr := "0"
	if tsDiscon {
		tsDisconStr = "1"
	}

	uri = replaceVars(uri, []string{
		"${PROTHOST}",
		"${SID}",
		"${FMT}",
		"${TSDISCON}",
		"${BITRATE}",
	}, []string{
		ctx.ServerProtocolHost,
		url.QueryEscape(ctx.SessionId),
		fmt,
		tsDisconStr,
		strconv.Itoa(bitrate),
	})
	return
}

// SessionGetURLCookies is used to obtain the required cookies for a given URL, this can be important for load balancing
func SessionGetURLCookies(ctx *SessionContext, uri string) (cookies []*http.Cookie, err error) {
	URL, e := url.Parse(uri)
	if e != nil {
		err = e
		cookies = nil
		return
	}
	cookies = ctx.CookieJar.Cookies(URL)
	err = nil
	return
}

func pktToBitstream(videoCodecData av.VideoCodecData, pkt *av.Packet) (data []byte) {
	if videoCodecData.Type() == av.H264 {
		// Prepare the h264 bitstream
		h264CodecData := videoCodecData.(h264parser.CodecData)

		// Add SPS/PPS before each key frame
		if pkt.IsKeyFrame {
			data = append(data, h264parser.StartCodeBytes...)
			data = append(data, h264CodecData.SPS()...)
			data = append(data, h264parser.StartCodeBytes...)
			data = append(data, h264CodecData.PPS()...)
		}

		pktnalus, _ := h264parser.SplitNALUs(pkt.Data)
		for _, nalu := range pktnalus {
			data = append(data, h264parser.StartCodeBytes...)
			data = append(data, nalu...)
		}
	} else {
		data = pkt.Data
	}
	return
}

func uiStream(ctx *SessionContext, uri string) (err error) {
	var reader io.ReadCloser
	reader, err = httpGet(ctx.CookieJar, uri, ctx.shouldStopUI)
	if err != nil {
		err = fmt.Errorf("Failed HTTP request for UI streaming: %v", err)
	}
	defer reader.Close()

	demuxer := ts.NewDemuxer(reader)
	if demuxer == nil {
		err = errors.New("Failed to create MPEG2TS demuxer from reader, uri: " + uri)
		return
	}

	var videoCodecData av.VideoCodecData
	streams, _ := demuxer.Streams()
	for _, stream := range streams {
		if stream.Type().IsAudio() {
			//astream := stream.(av.AudioCodecData)
		} else if stream.Type().IsVideo() {
			videoCodecData = stream.(av.VideoCodecData)
		}
	}

	// Double buffer the packets, we read a frame from the network while the previous frame read is being rendered
	var pkts [2]av.Packet
	readIndex := -1
	writeIndex := 0
	errChan := make(chan error, 1)
	for {
		go func() {
			pkts[writeIndex], err = demuxer.ReadPacket()
			if err != nil {
				err = fmt.Errorf("UI streaming failed to demux packet: %v", err)
			}
			errChan <- err
		}()

		if readIndex >= 0 {
			var data []byte
			pkt := &pkts[readIndex]
			data = pktToBitstream(videoCodecData, pkt)
			err = ctx.appflingerListener.OnUIFrame(pkt.IsKeyFrame, pkt.IsKeyFrame, int(pkt.Idx), int(pkt.CompositionTime), int(pkt.Time), data)
			if err != nil {
				err = fmt.Errorf("UI frame listener failed: %v", err)
				return
			}
		}

		// Wait for reading from the http request to complete
		select {
		case <-ctx.shouldStopUI:
			return
		case err = <-errChan:
			if err != nil {
				return
			}
		}

		readIndex = writeIndex
		writeIndex = 1 - writeIndex
	}

	return nil
}

func uiStreamRoutine(ctx *SessionContext, uri string) {
	err := uiStream(ctx, uri)
	if err != nil {
		log.Println("Failed to stream ui with error: ", err)
	}
	ctx.isUIStreaming = false
	ctx.isDone <- true
}

// SessionUIStreamStart is used to start streaming the UI, frames will be passed to OnUIFrame() in the AppFlinger listener
func SessionUIStreamStart(ctx *SessionContext, format string, tsDiscon bool, bitrate int) (err error) {
	if format != UI_FMT_TS_H264 {
		return fmt.Errorf("Only format %s is supported by the SDK", UI_FMT_TS_H264)
	}

	uri, e := SessionGetUIURL(ctx, format, tsDiscon, bitrate)
	if e != nil {
		return e
	}

	if ctx.isUIStreaming {
		return errors.New("UI is already streaming")
	}

	ctx.isUIStreaming = true
	go uiStreamRoutine(ctx, uri)
	return nil
}

// SessionUIStreamStop is used to stop streaming the UI
func SessionUIStreamStop(ctx *SessionContext) (err error) {
	if !ctx.isUIStreaming {
		return errors.New("UI is not streaming")
	}
	ctx.shouldStopUI <- true
	<-ctx.isDone
	return nil
}
