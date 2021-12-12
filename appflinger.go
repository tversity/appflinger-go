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
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/codec/h264parser"
	"github.com/nareix/joy4/format/ts"
	"golang.org/x/net/publicsuffix"
)

const (
	DEBUG_MODE = true
	// When DEBUG_MODE set to true and the UI stream requested with one of image formats, 
	// UI images are being saved as files in the folder specified in TEST_IMGSTREAM_DIR. 
	// The folder is being created (or re-created) under the current execution path when client starts.    
	TEST_IMGSTREAM_DIR = "testImgStream"

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
	_SESSION_START_URL            = "${PROTHOST}/osb/session/start?browser_url=${BURL}"
	_SESSION_STOP_URL             = "${PROTHOST}/osb/session/stop?session_id=${SID}"
	_SESSION_EVENT_URL            = "${PROTHOST}/osb/session/event?session_id=${SID}&type=${TYPE}"
	_SESSION_CONTROL_URL          = "${PROTHOST}/osb/session/control?session_id=${SID}"
	_SESSION_CONTROL_RESPONSE_URL = "${PROTHOST}/osb/session/control/response?session_id=${SID}"
	_SESSION_UI_URL               = "${PROTHOST}/osb/session/ui?session_id=${SID}&fmt=${FMT}&ts_discon=${TSDISCON}"

	// Keyboard codes for injecting events
	KEY_UP        = 0x26
	KEY_DOWN      = 0x28
	KEY_LEFT      = 0x25
	KEY_RIGHT     = 0x27
	KEY_ENTER     = 0xd
	KEY_BACKSPACE = 0x8
	KEY_ESCAPE    = 0x1b

	// Media formats supported for UI stream.
	// Video stream format contains two parts separated by semicolon, representing container followed by codec.
	UI_FMT_TS_H264  = "mp2t;h264"
	UI_FMT_MP4_H264 = "mp4;h264"
	UI_FMT_WEBM_VP8 = "webm;vp8"
	UI_FMT_WEBM_VP9 = "webm;vp9"
	UI_FMT_MPD_TS   = "mpd;mp2"
	UI_FMT_MPD_MP4  = "mpd;mp4"
	UI_FMT_MPD_WEBM = "mpd;webm"
	// Image stream format can contain one or two parts separated by semicolon. The first is the RGB image and the second is the alpha image. 
	// The alpha image is optional part of UI stream, it's not included in some formats.
	// We parse image stream format before sending to the server, since the server takes the alpha format in a separate query string argument. 
	// This is in contrast to the video container and codec which are passed to the server as is (after validation).
	UI_FMT_JPEG				= "jpeg"
	UI_FMT_PNG				= "png"
	UI_FMT_JPEG_ALPHA_JPEG	= "jpeg;jpeg"
	UI_FMT_JPEG_ALPHA_PNG	= "jpeg;png"
	UI_FMT_JPEG_ALPHA_PNG8	= "jpeg;png8"
	UI_FMT_JPEG_ALPHA_PNG32	= "jpeg;png32"

	// MSE AppendMode enum
	MSE_APPEND_MODE_SEGMENTS = 0
	MSE_APPEND_MODE_SEQUENCE = 1

	// EME MediaKeysRequirement enum
	EME_MEDIA_KEYS_REQUIRED    = "required"
	EME_MEDIA_KEYS_OPTIONAL    = "optional"
	EME_MEDIA_KEYS_NOT_ALLOWED = "not-allowed"

	// EME MediaKeySessionType enum
	EME_MEDIA_KEYS_SESSION_TEMPORARY          = "temporary"
	EME_MEDIA_KEYS_SESSION_PERSISTENT_LICENSE = "persistent-license"

	// Do not read more than this number of bytes as a safety mechanism against attacks, etc.
	_HTTP_MAX_RESPONSE_SIZE = 10000000
)

// Allowed formats for video streaming
var _ALLOWED_UI_VIDEO_FMT = map[string]bool{
	UI_FMT_TS_H264:  true,
	UI_FMT_MP4_H264: true,
	UI_FMT_WEBM_VP8: true,
	UI_FMT_WEBM_VP9: true,
	UI_FMT_MPD_TS:   true,
	UI_FMT_MPD_MP4:  true,
	UI_FMT_MPD_WEBM: true,
}

// Allowed formats for image streaming
var _ALLOWED_UI_IMAGE_FMT = map[string]bool{
	UI_FMT_PNG:      			true,
	UI_FMT_JPEG:     			true,
	UI_FMT_JPEG_ALPHA_JPEG:		true, // The actual alpha JPEG delivered will typically be grayscale JPEG.
	UI_FMT_JPEG_ALPHA_PNG:		true,
	UI_FMT_JPEG_ALPHA_PNG8:		true,
	UI_FMT_JPEG_ALPHA_PNG32:	true,
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
	Rate    string // Used in SetRate()
	Volume  string // Used in SetVolume()

	// Used in addSourceBuffer() and various other MSE related functions
	SourceId string
	Type     string

	// Used in appendBuffer()
	AppendWindowStart string
	AppendWindowEnd   string
	BufferId          string
	BufferOffset      string
	BufferLength      string

	// Used in setAppendMode()
	Mode string

	// Used in setAppendTimestampOffset()
	TimestampOffset string

	// Used in removeBufferRange()
	Start string
	End   string

	// Used in changeSourceBufferType()
	MimeType string

	// Used in loadResource()
	ResourceId     string
	Url            string
	Method         string
	Headers        string
	ByteRange      string
	SequenceNumber string

	// Used in setRect()
	X      string
	Y      string
	Width  string
	Height string

	// Used in requestKeySystem()
	KeySystem               string
	SupportedConfigurations []EMEMediaKeySystemConfiguration

	// Used in cdmCreate()
	SecurityOrigin             string
	AllowDistinctiveIdentifier string
	AllowPersistentState       string

	// Used in cdmSetServerCertificate()
	CdmId string

	// Used in cdmSessionCreate()
	SessionType  string
	InitDataType string

	// Used in cdmSessionUpdate()
	CdmSessionId string
}

type VideoStateChangeNotifcation struct {
	Type         string  `json:"type"` // "videostatechange"
	ReadyState   int     `json:"readyState"`
	NetworkState int     `json:"networkState"`
	Paused       string  `json:"paused"`
	Seeking      string  `json:"seeking"`
	Duration     float64 `json:"duration"`
	Time         float64 `json:"time"`
	VideoWidth   int     `json:"videoWidth"`
	VideoHeight  int     `json:"videoHeight"`
}

// MediaKeySystemMediaCapability as per EME spec
type EMEMediaKeySystemMediaCapability struct {
	ContentType string `json:"contentType"`
	Robustness  string `json:"robustness"`
}

// MediaKeySystemConfiguration as per EME spec
type EMEMediaKeySystemConfiguration struct {
	Label                 string                             `json:"label"`
	InitDataTypes         []string                           `json:"initDataTypes"`
	AudioCapabilities     []EMEMediaKeySystemMediaCapability `json:"audioCapabilities"`
	VideoCapabilities     []EMEMediaKeySystemMediaCapability `json:"videoCapabilities"`
	DistinctiveIdentifier string                             `json:"distinctiveIdentifier"` // One of EME_MEDIA_KEYS_REQUIRED, EME_MEDIA_KEYS_OPTIONAL, EME_MEDIA_KEYS_NOT_ALLOWED
	PersistentState       string                             `json:"persistentState"`       // One of EME_MEDIA_KEYS_REQUIRED, EME_MEDIA_KEYS_OPTIONAL, EME_MEDIA_KEYS_NOT_ALLOWED
	SessionTypes          []string                           `json:"sessionTypes"`          // Array values are one of EME_MEDIA_KEYS_SESSION_TEMPORARY, EME_MEDIA_KEYS_SESSION_PERSISTENT_LICENSE
}

// RequestKeySystemResult is the returned result by RequestKeySystem()
type RequestKeySystemResult EMEMediaKeySystemConfiguration

// LoadResourceResult is the returned result by LoadResource() in the AppflingerListener interface
type LoadResourceResult struct {
	Code         string
	Headers      string
	BufferId     string
	BufferLength int
	Payload      []byte
}

// UIImageHeader represents the json header preceding each image in the UI image stream.
// The image stream have this header in JSON format terminated by two newline characters and followed by the image bytes 
// and optionally followed by the alpha image bytes (depending on whether alpha was requested).
// Multiple such header and images are provided until a header is reached with the IsFrame field set to true, which indicates that we now have a complete frame. 
// All the images comprising a single frame should be made visible simultaneously for rendering to be correct.
type UIImageHeader struct {
	X 			int `json:"x,string"` 			// x position of image on the screen
	Y 			int `json:"y,string"` 			// y position of image on the screen
	Width		int	`json:"width,string"`		// image width
	Height 		int	`json:"height,string"`		// image height
	Size		int	`json:"size,string"`		// the size in bytes of the entire binary payload
	AlphaSize	int	`json:"alphaSize,string"`	// the size of the alpha image
	IsFrame		int	`json:"frame,string"`		// 0|1 indicating a completed video frame
}

type UIImage struct {
	Header		*UIImageHeader
	Img			[]byte
	AlphaImg	[]byte
}

// TimeIntervalArrays is the returned result by GetBuffered() and GetSeekable() in the AppflingerListener interface
type TimeIntervalArrays struct {
	Start []float64
	End   []float64
}

type GetBufferedResult TimeIntervalArrays
type GetSeekableResult TimeIntervalArrays

// Added this set of methods since gomobile cannot generate bindings to arrays

func (result *TimeIntervalArrays) GetLength() int {
	if len(result.Start) != len(result.End) {
		log.Println("Internal error, array length mismatch")
	}
	return len(result.Start)
}
func (result *TimeIntervalArrays) SetLength(length int) {
	result.Start = make([]float64, length)
	result.End = make([]float64, length)
}
func (result *TimeIntervalArrays) GetStart(index int) float64 {
	return result.Start[index]
}
func (result *TimeIntervalArrays) SetStart(index int, value float64) {
	result.Start[index] = value
}
func (result *TimeIntervalArrays) GetEnd(index int) float64 {
	return result.End[index]
}
func (result *TimeIntervalArrays) SetEnd(index int, value float64) {
	result.End[index] = value
}

// AppflingerListener is the interface a client needs to implement in order to process the control channel commands.
// An example is available under examples/stub.go. The "AppFlinger API and Client Integration Guide"
// describes the control channel operation and its various commands in detail.
type AppflingerListener interface {

	// Control Channel functions - media related

	Load(sessionId string, instanceId string, url string) (err error)
	CancelLoad(sessionId string, instanceId string) (err error)
	Pause(sessionId string, instanceId string) (err error)
	Play(sessionId string, instanceId string) (err error)
	Seek(sessionId string, instanceId string, time float64) (err error)
	GetPaused(sessionId string, instanceId string) (paused bool, err error)
	GetSeeking(sessionId string, instanceId string) (seeking bool, err error)
	GetDuration(sessionId string, instanceId string) (duration float64, err error)
	GetCurrentTime(sessionId string, instanceId string) (time float64, err error)
	GetNetworkState(sessionId string, instanceId string) (networkState int, err error)
	GetReadyState(sessionId string, instanceId string) (readyState int, err error)
	GetSeekable(sessionId string, instanceId string, result *GetSeekableResult) (err error)
	GetBuffered(sessionId string, instanceId string, result *GetBufferedResult) (err error)
	SetRect(sessionId string, instanceId string, x int, y int, width int, height int) (err error)
	SetVisible(sessionId string, instanceId string, visible bool) (err error)
	SetRate(sessionId string, instanceId string, rate float64) (err error)
	SetVolume(sessionId string, instanceId string, volume float64) (err error)

	// Control Channel functions - MSE related

	AddSourceBuffer(sessionId string, instanceId string, sourceId string, mimeType string) (err error)
	RemoveSourceBuffer(sessionId string, instanceId string, sourceId string) (err error)
	AbortSourceBuffer(sessionId string, instanceId string, sourceId string) (err error)
	AppendBuffer(sessionId string, instanceId string, sourceId string, appendWindowStart float64, appendWindowEnd float64, bufferId string, bufferOffset int,
		bufferLength int, payload []byte, result *GetBufferedResult) (err error)
	SetAppendMode(sessionId string, instanceId string, sourceId string, mode int) (err error)
	SetAppendTimestampOffset(sessionId string, instanceId string, sourceId string, timestampOffset float64) (err error)
	RemoveBufferRange(sessionId string, instanceId string, sourceId string, start float64, end float64) (err error)
	ChangeSourceBufferType(sessionId string, instanceId string, sourceId string, mimeType string) (err error)

	// Control Channel functions - client side XHR

	// TODO what about cancelLoadResource()? We need to make LoadResource cancellable
	LoadResource(sessionId string, url string, method string, headers string, resourceId string, byteRangeStart int, byteRangeEnd int,
		sequenceNumber int, payload []byte, result *LoadResourceResult) (err error)
	DeleteResource(sessionId string, BufferId string) (err error)

	// Control Channel functions - EME related
	// Note that eventInstanceId is for sending events which are associated with a given CDM session. It is serves
	// the same purpose as cdmSessionId but is needed before cdmSessionId exists.
	// TODO maybe get rid of cdmSessionId and just rename eventInstanceId to cdmSessionId
	// The instanceId used above and in SetCdm() is different, it is the instance of the media player (more than one may exist)
	RequestKeySystem(sessionId string, keySystem string, supportedConfigurations []EMEMediaKeySystemConfiguration, result *RequestKeySystemResult) (err error)
	CdmCreate(sessionId string, keySystem string, securityOrigin string, allowDistinctiveIdentifier bool, allowPersistentState bool) (cdmId string, err error)
	CdmSetServerCertificate(sessionId string, cdmId string, payload []byte) (err error)
	CdmSessionCreate(sessionId string, eventInstanceId string, cdmId string, sessionType string, initDataType string, payload []byte) (cdmSessionId string, expiration float64, err error)
	CdmSessionUpdate(sessionId string, eventInstanceId string, cdmId string, cdmSessionId string, payload []byte) (err error)
	CdmSessionLoad(sessionId string, eventInstanceId string, cdmId string, cdmSessionId string) (loaded bool, expiration float64, err error)
	CdmSessionRemove(sessionId string, eventInstanceId string, cdmId string, cdmSessionId string) (err error)
	CdmSessionClose(sessionId string, eventInstanceId string, cdmId string, cdmSessionId string) (err error)
	SetCdm(sessionId string, instanceId string, cdmId string) (err error)

	// Control Channel functions - General

	SendMessage(sessionId string, message string) (result string, err error)
	OnPageLoad(sessionId string) (err error)
	OnAddressBarChanged(sessionId string, url string) (err error)
	OnTitleChanged(sessionId string, title string) (err error)
	OnPageClose(sessionId string) (err error)

	// Misc Go SDK functions

	OnUIVideoFrame(sessionId string, isCodecConfig bool, isKeyFrame bool, idx int, pts int, dts int, data []byte) (err error)
	OnUIImageFrame(sessionId string, imgData *UIImage) (err error)
}

var (
	ErrInterrupted  = errors.New("Aborting due to interrupt")
	globalRequestId = 0
	sessionIdToCtx  = make(map[string]*SessionContext)
)

func getRequestId() string {
	globalRequestId++
	return strconv.Itoa(globalRequestId)
}

func boolToStr(val bool) string {
	if val {
		return "1"
	} else {
		return "0"
	}
}

func strToBool(val string) bool {
	return val == "true" || val == "yes" || val == "1"
}

func replaceVars(str string, vars []string, vals []string) (result string) {
	result = str
	for i := range vars {
		result = strings.Replace(result, vars[i], vals[i], -1)
	}
	return
}

func marshalRPCNotification(sessionId string, requestId string, instanceId string, payload []byte) (notif []byte, err error) {
	jsonMap := make(map[string]interface{})
	jsonMap["service"] = "eventNotification"
	jsonMap["sessionId"] = sessionId
	jsonMap["requestId"] = requestId
	jsonMap["instanceId"] = instanceId

	if payload != nil {
		jsonMap["payloadSize"] = strconv.Itoa(len(payload))
	}

	notif, err = json.Marshal(jsonMap)
	if err != nil {
		err = fmt.Errorf("Error in JSON marshaling of %v, reason: %w", jsonMap, err)
	} else if payload != nil {
		// RPC message format requires messages to be terminated with "\n\n"
		notif = append(notif, []byte("\n\n")...)
		notif = append(notif, payload...)
	}
	return
}

func marshalRPCResponse(result map[string]interface{}, resultPayload []byte, respErr error) (resp []byte, err error) {
	if respErr == nil {
		result["result"] = "OK"
		if result["message"] == nil {
			result["message"] = ""
		}
		if resultPayload != nil {
			result["payloadSize"] = len(resultPayload)
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
			err = appf.Load(req.SessionId, req.InstanceId, req.URL)
		}
	} else if req.Service == "cancelLoad" {
		//log.Println("service: " + req.Service)
		if appf != nil {
			err = appf.CancelLoad(req.SessionId, req.InstanceId)
		}
	} else if req.Service == "play" {
		//log.Println("service: " + req.Service)
		if appf != nil {
			err = appf.Play(req.SessionId, req.InstanceId)
		}
	} else if req.Service == "pause" {
		//log.Println("service: " + req.Service)
		if appf != nil {
			err = appf.Pause(req.SessionId, req.InstanceId)
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
			err = appf.Seek(req.SessionId, req.InstanceId, time)
		}
	} else if req.Service == "getPaused" {
		//log.Println("service: " + req.Service)
		paused := false
		if appf != nil {
			paused, err = appf.GetPaused(req.SessionId, req.InstanceId)
		}
		if err == nil {
			result["paused"] = boolToStr(paused)
		}
	} else if req.Service == "getSeeking" {
		//log.Println("service: " + req.Service)
		seeking := false
		if appf != nil {
			seeking, err = appf.GetSeeking(req.SessionId, req.InstanceId)
		}
		if err == nil {
			result["seeking"] = boolToStr(seeking)
		}
	} else if req.Service == "getDuration" {
		//log.Println("service: " + req.Service)
		duration := float64(0)
		if appf != nil {
			duration, err = appf.GetDuration(req.SessionId, req.InstanceId)
		}
		if err == nil {
			result["duration"] = strconv.FormatFloat(duration, 'f', -1, 64)
		}
	} else if req.Service == "getCurrentTime" {
		//log.Println("service: " + req.Service)
		time := float64(0)
		if appf != nil {
			time, err = appf.GetCurrentTime(req.SessionId, req.InstanceId)
		}
		if err == nil {
			result["currentTime"] = strconv.FormatFloat(time, 'f', -1, 64)
		}
	} else if req.Service == "getSeekable" {
		//log.Println("service: " + req.Service)
		var getSeekableResult GetSeekableResult
		if appf != nil {
			err = appf.GetSeekable(req.SessionId, req.InstanceId, &getSeekableResult)
		}
		if err == nil {
			result["start"] = getSeekableResult.Start
			result["end"] = getSeekableResult.End
		}
	} else if req.Service == "getNetworkState" {
		//log.Println("service: " + req.Service)
		state := NETWORK_STATE_LOADED
		if appf != nil {
			state, err = appf.GetNetworkState(req.SessionId, req.InstanceId)
		}
		if err == nil {
			result["networkState"] = strconv.Itoa(state)
		}
	} else if req.Service == "getReadyState" {
		//log.Println("service: " + req.Service)
		state := READY_STATE_HAVE_ENOUGH_DATA
		if appf != nil {
			state, err = appf.GetReadyState(req.SessionId, req.InstanceId)
		}
		if err == nil {
			result["readyState"] = strconv.Itoa(state)
		}
	} else if req.Service == "getBuffered" {
		//log.Println("service: " + req.Service)
		// Time range of buffered portions, there can be gaps that are unbuffered hence
		// we are dealing with two arrays and not two scalars.
		var getBufferedResult GetBufferedResult
		if appf != nil {
			err = appf.GetBuffered(req.SessionId, req.InstanceId, &getBufferedResult)
		}
		if err == nil {
			if getBufferedResult.Start != nil && getBufferedResult.End != nil {
				result["start"] = getBufferedResult.Start
				result["end"] = getBufferedResult.End
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
			err = appf.SetRect(req.SessionId, req.InstanceId, int(x), int(y), int(width), int(height))
		}
	} else if req.Service == "setVisible" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.Visible))
		if appf != nil {
			err = appf.SetVisible(req.SessionId, req.InstanceId, strToBool(req.Visible))
		}
	} else if req.Service == "setRate" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.Rate))
		var rate float64
		rate, err = strconv.ParseFloat(req.Rate, 64)
		if err != nil {
			err = errors.New("Failed to parse float: " + req.Rate)
			log.Println(err)
			resp, err = marshalRPCResponse(result, resultPayload, err)
			return
		}

		if appf != nil {
			err = appf.SetRate(req.SessionId, req.InstanceId, rate)
		}
	} else if req.Service == "setVolume" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.Volume))
		var volume float64
		volume, err = strconv.ParseFloat(req.Volume, 64)
		if err != nil {
			err = errors.New("Failed to parse float: " + req.Volume)
			log.Println(err)
			resp, err = marshalRPCResponse(result, resultPayload, err)
			return
		}

		if appf != nil {
			err = appf.SetVolume(req.SessionId, req.InstanceId, volume)
		}
	} else if req.Service == "addSourceBuffer" {
		//log.Println(fmt.Sprintf("service: %s -- %s, %s", req.Service, req.SourceId, req.Type))
		if appf != nil {
			err = appf.AddSourceBuffer(req.SessionId, req.InstanceId, req.SourceId, req.Type)
		}
	} else if req.Service == "removeSourceBuffer" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.SourceId))
		if appf != nil {
			err = appf.RemoveSourceBuffer(req.SessionId, req.InstanceId, req.SourceId)
		}
	} else if req.Service == "abortSourceBuffer" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.SourceId))
		if appf != nil {
			err = appf.AbortSourceBuffer(req.SessionId, req.InstanceId, req.SourceId)
		}
	} else if req.Service == "setAppendMode" {
		//log.Println(fmt.Sprintf("service: %s -- %s, %s", req.Service, req.SourceId, req.Mode))
		var mode uint64
		mode, err = strconv.ParseUint(req.Mode, 10, 0)
		if err != nil {
			err = errors.New("Failed to parse integer: " + req.Mode)
			log.Println(err)
			resp, err = marshalRPCResponse(result, resultPayload, err)
			return
		}

		if appf != nil {
			err = appf.SetAppendMode(req.SessionId, req.InstanceId, req.SourceId, int(mode))
		}
	} else if req.Service == "setAppendTimestampOffset" {
		//log.Println(fmt.Sprintf("service: %s -- %s, %s", req.Service, req.SourceId, req.TimestampOffset))

		var timestampOffset float64
		timestampOffset, err = strconv.ParseFloat(req.TimestampOffset, 64)
		if err != nil {
			err = errors.New("Failed to parse float: " + req.TimestampOffset)
			log.Println(err)
			resp, err = marshalRPCResponse(result, resultPayload, err)
			return
		}

		if appf != nil {
			err = appf.SetAppendTimestampOffset(req.SessionId, req.InstanceId, req.SourceId, timestampOffset)
		}
	} else if req.Service == "removeBufferRange" {
		//log.Println(fmt.Sprintf("service: %s -- %s, %s", req.Service, req.SourceId, req.TimestampOffset))

		var start, end float64
		start, err = strconv.ParseFloat(req.Start, 64)
		if err != nil {
			err = errors.New("Failed to parse float: " + req.Start)
			log.Println(err)
			resp, err = marshalRPCResponse(result, resultPayload, err)
			return
		}
		end, err = strconv.ParseFloat(req.End, 64)
		if err != nil {
			err = errors.New("Failed to parse float: " + req.End)
			log.Println(err)
			resp, err = marshalRPCResponse(result, resultPayload, err)
			return
		}

		if appf != nil {
			err = appf.RemoveBufferRange(req.SessionId, req.InstanceId, req.SourceId, start, end)
		}
	} else if req.Service == "changeSourceBufferType" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.SourceId, req.MimeType))
		if appf != nil {
			err = appf.ChangeSourceBufferType(req.SessionId, req.InstanceId, req.SourceId, req.MimeType)
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
			err = appf.AppendBuffer(req.SessionId, req.InstanceId, req.SourceId, appendWindowStart, appendWindowEnd, req.BufferId,
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
			err = appf.LoadResource(req.SessionId, req.Url, req.Method, req.Headers, req.ResourceId,
				int(byteRange[0]), int(byteRange[1]), int(sequenceNumber), payload, &loadResourceResult)
			if err == nil {
				result["code"] = loadResourceResult.Code
				result["headers"] = loadResourceResult.Headers
				if req.ResourceId != "" {
					result["bufferId"] = loadResourceResult.BufferId
					result["bufferLength"] = strconv.Itoa(loadResourceResult.BufferLength)
				}
				resultPayload = loadResourceResult.Payload
			}
		}
	} else if req.Service == "deleteResource" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.BufferId))
		if appf != nil {
			err = appf.DeleteResource(req.SessionId, req.BufferId)
		}
	} else if req.Service == "requestKeySystem" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.KeySystem, req.supportedConfigurations))
		var requestKeySystemResult RequestKeySystemResult
		if appf != nil {
			err = appf.RequestKeySystem(req.SessionId, req.KeySystem, req.SupportedConfigurations, &requestKeySystemResult)
		}
		if err == nil {
			result["requestKeySystemResult"] = requestKeySystemResult
		}
	} else if req.Service == "cdmCreate" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.KeySystem, req.SecurityOrigin, req.AllowDistinctiveIdentifier, req.AllowPersistentState))
		cdmId := ""
		if appf != nil {
			cdmId, err = appf.CdmCreate(req.SessionId, req.KeySystem, req.SecurityOrigin, strToBool(req.AllowDistinctiveIdentifier), strToBool(req.AllowPersistentState))
		}
		if err == nil {
			result["cdmId"] = cdmId
		}
	} else if req.Service == "cdmSetServerCertificate" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.CdmId))
		if appf != nil {
			err = appf.CdmSetServerCertificate(req.SessionId, req.CdmId, payload)
		}
	} else if req.Service == "cdmSessionCreate" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.CdmId, req.SessionType, req.InitDataType))
		cdmSessionId := ""
		var expiration float64
		if appf != nil {
			cdmSessionId, expiration, err = appf.CdmSessionCreate(req.SessionId, req.InstanceId, req.CdmId, req.SessionType, req.InitDataType, payload)
		}
		if err == nil {
			result["cdmSessionId"] = cdmSessionId
			result["expiration"] = strconv.FormatFloat(expiration, 'f', -1, 64)
		}
	} else if req.Service == "cdmSessionUpdate" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.CdmId, req.CdmSessionId))
		if appf != nil {
			err = appf.CdmSessionUpdate(req.SessionId, req.InstanceId, req.CdmId, req.CdmSessionId, payload)
		}
	} else if req.Service == "cdmSessionLoad" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.CdmId, req.CdmSessionId))
		loaded := false
		var expiration float64
		if appf != nil {
			loaded, expiration, err = appf.CdmSessionLoad(req.SessionId, req.InstanceId, req.CdmId, req.CdmSessionId)
		}
		if err == nil {
			result["loaded"] = boolToStr(loaded)
			result["expiration"] = strconv.FormatFloat(expiration, 'f', -1, 64)
		}
	} else if req.Service == "cdmSessionRemove" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.CdmId, req.CdmSessionId))
		if appf != nil {
			err = appf.CdmSessionRemove(req.SessionId, req.InstanceId, req.CdmId, req.CdmSessionId)
		}
	} else if req.Service == "cdmSessionClose" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.CdmId, req.CdmSessionId))
		if appf != nil {
			err = appf.CdmSessionClose(req.SessionId, req.InstanceId, req.CdmId, req.CdmSessionId)
		}
	} else if req.Service == "setCdm" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.CdmId))
		if appf != nil {
			err = appf.SetCdm(req.SessionId, req.InstanceId, req.CdmId)
		}
	} else if req.Service == "sendMessage" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.Message))
		message := ""
		if appf != nil {
			message, err = appf.SendMessage(req.SessionId, req.Message)
		}
		if err == nil {
			result["message"] = message
		}
	} else if req.Service == "onPageLoad" {
		//log.Println("service: ", req.Service)
		if appf != nil {
			err = appf.OnPageLoad(req.SessionId)
		}
	} else if req.Service == "onAddressBarChanged" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.URL))
		if appf != nil {
			err = appf.OnAddressBarChanged(req.SessionId, req.URL)
		}
	} else if req.Service == "onTitleChanged" {
		//log.Println(fmt.Sprintf("service: %s -- %s", req.Service, req.Title))
		if appf != nil {
			err = appf.OnTitleChanged(req.SessionId, req.Title)
		}
	} else if req.Service == "onPageClose" {
		//log.Println("service: ", req.Service)
		if appf != nil {
			err = appf.OnPageClose(req.SessionId)
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
			err = ErrInterrupted
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
			err = ErrInterrupted
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

		// Empty messages are sent periodically to keep the connection open
		if jsonEndPos == 0 {
			httpRes.Body.Close()
			postMessage = nil
			continue
		}

		jsonEndPos += 2

		// Parse the response
		req := &controlChannelRequest{}
		err = json.Unmarshal(body[:jsonEndPos], req)
		if err != nil {
			// Check if need to abort in a non blocking way
			select {
			case <-ctx.shouldStopSession:
				err = ErrInterrupted
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

		payload := body[jsonEndPos:]
		if req.PayloadSize != "" || len(payload) > 0 {
			var payloadSize uint64
			payloadSize, err = strconv.ParseUint(req.PayloadSize, 10, 0)
			if err != nil {
				err = errors.New("Failed to parse payload size integer: " + req.PayloadSize)
			} else if uint64(len(payload)) != payloadSize {
				err = fmt.Errorf("Payload size mismatch, %d != %d", len(payload), payloadSize)
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

func httpReq(cookieJar *cookiejar.Jar, uri string, method string, body io.Reader, shouldStop chan bool) (io.ReadCloser, error) {
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

	var err error
	var httpReq *http.Request
	var httpRes *http.Response
	httpReq, err = http.NewRequest(method, uri, body)
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
			return nil, ErrInterrupted
		case err = <-errChan:
			if err != nil {
				return nil, err
			}
		}
	} else {
		httpRes, err = client.Do(httpReq)
		if err != nil {
			err = fmt.Errorf("HTTP request failed with error: %v, uri: %s", err, uri)
			return nil, err
		}
	}

	if httpRes.StatusCode != http.StatusOK {
		err = fmt.Errorf("HTTP request failed with status: %s, uri: %s", httpRes.Status, uri)
		httpRes.Body.Close()
		return nil, err
	}

	return httpRes.Body, nil
}

func httpGet(cookieJar *cookiejar.Jar, uri string, shouldStop chan bool) (io.ReadCloser, error) {
	return httpReq(cookieJar, uri, http.MethodGet, nil, shouldStop)
}

func httpPost(cookieJar *cookiejar.Jar, uri string, body []byte, shouldStop chan bool) (io.ReadCloser, error) {
	if body == nil {
		return httpReq(cookieJar, uri, http.MethodPost, nil, shouldStop)
	} else {
		return httpReq(cookieJar, uri, http.MethodPost, bytes.NewReader(body), shouldStop)
	}

}

func apiReq(cookieJar *cookiejar.Jar, uri string, body []byte, shouldStop chan bool, resp interface{}) (err error) {
	var reader io.ReadCloser
	var e error
	if body == nil {
		reader, e = httpReq(cookieJar, uri, http.MethodGet, nil, shouldStop)
	} else {
		reader, e = httpReq(cookieJar, uri, http.MethodPost, bytes.NewReader(body), shouldStop)
	}
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
	if err != nil && err != ErrInterrupted {
		// We do not print the err since it can lead to a crash in
		// net.(*OpError).Error(0xc000054c40, 0x717a68, 0xc000180000)
		// this is in file/line go/src/net/net.go:460
		// the crash code is: s += " " + e.Source.String()
		// Although it is encloses in: if e.Source != nil
		log.Println("Control channel connection ended with error")
	}
}

// SessionStart is used to start a new session or navigate an existing one to a new address.
// The arguments to this function are as per the description of the /osb/session/start API in
// the "AppFlinger API and Client Integration Guide".
func SessionStart(serverProtocolHost string, sessionId string, browserURL string, pullMode bool, isVideoPassthru bool, browserUIOutputURL string,
	videoStreamURL string, width int, height int, appf AppflingerListener) (ctx *SessionContext, err error) {
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
	if width > 0 && width <= 3840 {
		uri += "&width=${WIDTH}"
	}
	if height > 0 && height <= 2160 {
		uri += "&height=${HEIGHT}"
	}

	uri = replaceVars(uri, []string{
		"${PROTHOST}",
		"${BURL}",
		"${VURL}",
		"${UURL}",
		"${SID}",
		"${WIDTH}",
		"${HEIGHT}",
	}, []string{
		serverProtocolHost,
		url.QueryEscape(browserURL),
		url.QueryEscape(videoStreamURL),
		url.QueryEscape(browserUIOutputURL),
		url.QueryEscape(sessionId),
		strconv.Itoa(width),
		strconv.Itoa(height),
	})

	// Make the request
	// We get here a struct with the data returned from the server (namely the session id)
	resp := &sessionStartResp{}
	err = apiReq(cookieJar, uri, nil, nil, resp)
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
	sessionIdToCtx[ctx.SessionId] = ctx
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
	err = apiReq(ctx.CookieJar, uri, nil, nil, nil)
	if err != nil {
		log.Println("Failed to stop session: ", err)
		return
	}
	return
}

func SessionGetSessionId(ctx *SessionContext) (sessionId string, err error) {
	sessionId = ctx.SessionId
	err = nil
	return
}

func SessionGetSessionContext(sessionId string) (ctx *SessionContext, err error) {
	ctx = sessionIdToCtx[sessionId]
	err = nil
	return
}

// SessionSendEvent is used to inject input into a session.
func SessionSendEvent(ctx *SessionContext, eventType string, code int, char rune, x int, y int) (err error) {
	// Construct the URL
	uri := _SESSION_EVENT_URL
	eventType = strings.ToLower(eventType)
	if eventType == "key" || eventType == "keydown" || eventType == "keyup" {
		uri += "&code=${KEYCODE}"
	} else if eventType == "click" {
		uri += "&x=${X}&y=${Y}"
	} else {
		err = errors.New("Invalid event type: " + eventType)
		return
	}

	if char > 0 {
		uri += "&char=${CHAR}"
	}

	codeString := strconv.Itoa(code)

	uri = replaceVars(uri, []string{
		"${PROTHOST}",
		"${SID}",
		"${TYPE}",
		"${KEYCODE}",
		"${CHAR}",
		"${X}",
		"${Y}",
	}, []string{
		ctx.ServerProtocolHost,
		url.QueryEscape(ctx.SessionId),
		eventType,
		codeString,
		codeString,
		strconv.Itoa(x),
		strconv.Itoa(y),
	})

	// Make the request
	err = apiReq(ctx.CookieJar, uri, nil, ctx.shouldStopSession, nil)
	return
}

func isImageFormat(fmt string) (result bool, imgFormat string, alphaFormat string, err error) {
	if _ALLOWED_UI_IMAGE_FMT[fmt] {
		result = true
		fmtParts := strings.Split(fmt, ";")
		if len(fmtParts) != 2 {
			return false, "", "", errors.New("Invalid format: " + fmt)
		}
		imgFormat = fmtParts[0]
		if len(fmtParts) > 1 {
			alphaFormat = fmtParts[1]
		}
	}
	return result, imgFormat, alphaFormat, nil
}

// SessionGetUIURL is used to obtain the HTTP URL from which the browser UI can be streamed.
// Note that one should also consider the cookies for that URL before making the HTTP request.
func SessionGetUIURL(ctx *SessionContext, fmt string, tsDiscon bool, bitrate int) (uri string, err error) {
	err = nil

	if !_ALLOWED_UI_VIDEO_FMT[fmt] && !_ALLOWED_UI_IMAGE_FMT[fmt] {
		err = errors.New("Invalid format: " + fmt)
		return
	}
	// Construct the URL
	uri = _SESSION_UI_URL

	isImageStream, imgFormat, alphaFormat, err := isImageFormat(fmt)
	if err != nil {
		return "", err
	}
	if isImageStream && alphaFormat != "" {
		uri += "&alpha=" + alphaFormat
	}
	
	if bitrate > 0 {
		uri += "&bitrate=${BITRATE}"
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
		imgFormat,
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

// writeFile is used for testing purposes only
func writeFile (fname string, bytes []byte) error {
	outFile, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer outFile.Close()
	outWriter := bufio.NewWriter(outFile)
	if outWriter == nil {
		return errors.New("Failed to create a buffered io writer")
	}
	_, err = outWriter.Write(bytes)
	if err != nil {
		return errors.New("Failed to write file " + fname)
	}
	outWriter.Flush()
	return err
}

func concatBytes(slice, data []byte) []byte {
	if data != nil && len(data) > 0 {
		if slice == nil {
			return data
		}
		l := len(slice)
		if l + len(data) > cap(slice) {  // reallocate
			// Allocate double what's needed, for future growth.
			newSlice := make([]byte, (l+len(data))*2)
			copy(newSlice, slice)
			slice = newSlice
		}
		slice = slice[0:l+len(data)]
		copy(slice[l:], data)
	}
    return slice
}

func wrapReadError(errFormat string, e error) (err error) {
	if DEBUG_MODE {
		fmt.Printf("--- wrapReadError: e = %+#v \n %v \n", e, e)
	}
	var opError *net.OpError
	if errors.As(e, &opError) {
		if opError.Op == "read" && strings.Contains(e.Error(), "closed network connection") {
			err = ErrInterrupted
			return
		}
	}
	err = fmt.Errorf(errFormat, e)
	return
}

func readImage(br *bufio.Reader, imgData *UIImage, ctx *SessionContext) (err error) {
	
	var data []byte // is used to gather image header json when receiving in chunks

	for {
		if imgData.Header == nil {
			portion, e := br.ReadBytes('\n') // including the delimiter
			if e != nil {
				err = wrapReadError("failed to read image header: %v", e)
				return 
			}
			data = concatBytes(data, portion)

			// Check if data followed by the second \n
			b, e := br.ReadByte()
			if e != nil {
				continue
			}
			if b != '\n' {
				data = concatBytes(data, []byte{b})
				continue
			}

			data = data[:len(data) - 1] // remove trailing \n

			if len(data) == 0 { // 'keep alive' request
				continue
			}

			jsonHeader := UIImageHeader{}
			err = json.Unmarshal(data, &jsonHeader)
			if err != nil {
				err = fmt.Errorf("failed to unmarshal image header %#v, reason: %v", data, err)
				return
			}
			imgData.Header = &jsonHeader
			data = nil
		}

		imgBytes := make([]byte, imgData.Header.Size - imgData.Header.AlphaSize - len(imgData.Img))
		_, err = io.ReadFull(br, imgBytes)
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				imgData.Img = concatBytes(imgData.Img, imgBytes)
				continue
			}
			err = wrapReadError("unable to read image chunk: %v", err)
			return 
		}
		imgData.Img = concatBytes(imgData.Img, imgBytes)

		if imgData.Header.AlphaSize > 0 {
			imgBytes := make([]byte, imgData.Header.AlphaSize - len(imgData.AlphaImg))
			_, err = io.ReadFull(br, imgBytes)
			if err != nil {
				if err == io.ErrUnexpectedEOF {
					imgData.AlphaImg = concatBytes(imgData.AlphaImg, imgBytes)
					continue
				}
				err = wrapReadError("unable to read alpha chunk: %v", err)
				return 
			}
			imgData.AlphaImg = concatBytes(imgData.AlphaImg, imgBytes)
		}
		return
	}
}

func uiImageStream(ctx *SessionContext, uri string, format string) (err error) {
	var reader io.ReadCloser
	reader, err = httpGet(ctx.CookieJar, uri, ctx.shouldStopUI)
	if err != nil {
		err = fmt.Errorf("Failed HTTP request for UI streaming: %v", err)
		return
	}
	defer reader.Close()

	if DEBUG_MODE {
		os.RemoveAll(TEST_IMGSTREAM_DIR)
		os.MkdirAll(TEST_IMGSTREAM_DIR, os.ModePerm)
	}

	// Double buffer the images, we read a frame from the network while the previous frame read is being rendered
	var images [2]*UIImage
	readIndex := -1
	writeIndex := 0

	br := bufio.NewReader(reader)
	errChan := make(chan error, 1)
	stopped := false

	for i := 0;; i++ {

		// Prevent another read operation after ctx.shouldStopUI received true 
		// and before the previous read is actually cancelled which allows to complete ui streaming 
		if !stopped {
			go func() {
				defer func() { errChan <- err } ()

				var imgData *UIImage
				if images[writeIndex] != nil {
					imgData = images[writeIndex]
				} else {
					imgData = &UIImage{}
				}

				err = readImage(br, imgData, ctx)
				if err != nil {
					return
				}
				images[writeIndex] = imgData

				if DEBUG_MODE {
					fmt.Printf("--- uiImageStream: imgData.Header = %+#v \n", imgData.Header)
					// fmt.Printf("--- uiImageStream: imgData.Img = %+#v \n", imgData.Img)
					fmtParts := strings.Split(format, ";")
					if imgData.Header.AlphaSize == 0 && len(fmtParts) != 1 || imgData.Header.AlphaSize > 0 && len(fmtParts) != 2 {
						err = fmt.Errorf("invalid UI image stream format: %v", format)
						return 
					}
					err = writeFile(TEST_IMGSTREAM_DIR + "/out" + strconv.Itoa(i) + "." + fmtParts[0], imgData.Img)
					if err != nil {
						log.Println(err)
					}
					if imgData.Header.AlphaSize > 0 {
						err = writeFile(TEST_IMGSTREAM_DIR + "/out" + strconv.Itoa(i) + "alpha." + fmtParts[1], imgData.AlphaImg)
						if err != nil {
							log.Println(err)
						}
					}
				}
			}()

			if readIndex >= 0 {
				if images[readIndex] == nil {
					// should never happen
					err = fmt.Errorf("UI frame listener failed: image is not obtained")
					return
				}
				err = ctx.appflingerListener.OnUIImageFrame(ctx.SessionId, images[readIndex])
				if err != nil {
					err = fmt.Errorf("UI frame listener failed: %v", err)
					return
				}
				images[readIndex].Header = nil
				images[readIndex].Img = nil
				images[readIndex].AlphaImg = nil
			}
		}

		select {
		case <-ctx.shouldStopUI:
			reader.Close()
			stopped = true
		case err = <-errChan:
			if err != nil {
				return
			}
		}

		readIndex = writeIndex
		writeIndex = 1 - writeIndex
	}
}

func uiVideoStream(ctx *SessionContext, uri string) (err error) {
	var reader io.ReadCloser
	reader, err = httpGet(ctx.CookieJar, uri, ctx.shouldStopUI)
	if err != nil {
		err = fmt.Errorf("Failed HTTP request for UI streaming: %v", err)
		return
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
	stopped := false

	for {
		// Prevent another read operation after ctx.shouldStopUI received true 
		// and before the previous read is actually cancelled which allows to complete ui streaming 
		if !stopped {
			go func() {
				pkts[writeIndex], err = demuxer.ReadPacket()
				if err != nil {
					err = wrapReadError("UI streaming failed to demux packet: %v", err)
				}
				errChan <- err
			}()

			if readIndex >= 0 {
				var data []byte
				pkt := &pkts[readIndex]
				data = pktToBitstream(videoCodecData, pkt)
				err = ctx.appflingerListener.OnUIVideoFrame(ctx.SessionId, pkt.IsKeyFrame, pkt.IsKeyFrame, int(pkt.Idx), int(pkt.CompositionTime), int(pkt.Time), data)
				if err != nil {
					err = fmt.Errorf("UI frame listener failed: %v", err)
					return
				}
			}
		}
		// Wait for reading from the http request to complete
		select {
		case <-ctx.shouldStopUI:
			reader.Close()
			stopped = true
		case err = <-errChan:
			if err != nil {
				return
			}
		}

		readIndex = writeIndex
		writeIndex = 1 - writeIndex
	}
}

func uiStreamRoutine(ctx *SessionContext, uri string, format string) {
	var err error
	if _ALLOWED_UI_IMAGE_FMT[format] {
		err = uiImageStream(ctx, uri, format)
	} else if _ALLOWED_UI_VIDEO_FMT[format] {
		err = uiVideoStream(ctx, uri)
	} else {
		err = fmt.Errorf("unsupported format %v", format)
	}
	if err != nil && err != ErrInterrupted {
		log.Println("Failed to stream ui with error: ", err)
	}
	ctx.isUIStreaming = false
	ctx.isDone <- true
}

// SessionUIStreamStart is used to start streaming the UI, frames will be passed to one of OnUIVideoFrame() or OnUIImageFrame() in the AppFlinger listener
func SessionUIStreamStart(ctx *SessionContext, format string, tsDiscon bool, bitrate int) (err error) {
	uri, e := SessionGetUIURL(ctx, format, tsDiscon, bitrate)
	if e != nil {
		return e
	}
	if DEBUG_MODE {
		fmt.Printf("--- SessionUIStreamStart: uri = %+v \n", uri)
	}

	if ctx.isUIStreaming {
		return errors.New("UI is already streaming")
	}

	ctx.isUIStreaming = true
	go uiStreamRoutine(ctx, uri, format)
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

func SessionSendNotification(ctx *SessionContext, instanceId string, payload []byte) (err error) {
	// Construct the URL
	uri := _SESSION_CONTROL_RESPONSE_URL
	uri = replaceVars(uri, []string{
		"${PROTHOST}",
		"${SID}",
	}, []string{
		ctx.ServerProtocolHost,
		url.QueryEscape(ctx.SessionId),
	})

	// Make the request
	err = apiReq(ctx.CookieJar, uri, payload, ctx.shouldStopSession, nil)
	return
}

func NotificationCreateVideoStateChange(readyState int, networkState int, paused bool, seeking bool,
	duration float64, time float64, videoWidth int, videoHeight int) ([]byte, error) {
	notif := VideoStateChangeNotifcation{
		Type:         "videostatechange",
		ReadyState:   readyState,
		NetworkState: networkState,
		Paused:       boolToStr(paused),
		Seeking:      boolToStr(seeking),
		Duration:     duration,
		Time:         time,
		VideoWidth:   videoWidth,
		VideoHeight:  videoHeight,
	}
	json, err := json.Marshal(notif)
	if err != nil {
		return nil, fmt.Errorf("Error in JSON marshaling of %v, reason: %w", notif, err)
	}

	return json, nil
}

func SessionSendNotificationVideoStateChange(ctx *SessionContext, instanceId string, readyState int,
	networkState int, paused bool, seeking bool, duration float64, time float64, videoWidth int, videoHeight int) (err error) {
	notif, err := NotificationCreateVideoStateChange(readyState, networkState, paused, seeking, duration, time,
		videoWidth, videoHeight)
	if notif == nil || err != nil {
		err = fmt.Errorf("Failed to create the notification: %w", err)
		return
	}

	notif, err = marshalRPCNotification(ctx.SessionId, getRequestId(), instanceId, notif)
	if err != nil {
		err = fmt.Errorf("Failed to marshal the notification RPC json, error: %w", err)
		return
	}
	err = SessionSendNotification(ctx, instanceId, notif)
	return
}

func NotificationCreateEncrypted(initDataType string, payload []byte) []byte {
	return nil
}

func NotificationCreateCdmSessionMessage(messageType string, payload []byte) []byte {
	return nil
}

func NotificationCreateCdmSessionKeyStatusesChange(payload []byte) []byte {
	return nil
}
