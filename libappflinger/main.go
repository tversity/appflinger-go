package main

import (
	// #include "callbacks.h"
	"C"

	"github.com/tversity/appflinger-go"
)
import "log"

var (
	err        error
	ctxHandles []*appflinger.SessionContext
)

func GoBool(val C.int) bool {
	if val == 0 {
		return false
	}
	return true
}

func CBool(val bool) C.int {
	if val {
		return 1
	}
	return 0
}

//export SessionStart
func SessionStart(serverProtocolHost *C.char, sessionId *C.char, browserURL *C.char, pullMode C.int,
	isVideoPassthru C.int, browserUIOutputURL *C.char, videoStreamURL *C.char, cb *C.appflinger_callbacks_t) C.int {
	stub := NewAppflingerListener(cb)
	var ctx *appflinger.SessionContext
	ctx, err = appflinger.SessionStart(C.GoString(serverProtocolHost), C.GoString(sessionId),
		C.GoString(browserURL), GoBool(pullMode), GoBool(isVideoPassthru),
		C.GoString(browserUIOutputURL), C.GoString(videoStreamURL), stub)
	if err != nil {
		log.Println(err)
		return -1
	}
	ctxHandles = append(ctxHandles, ctx)
	return C.int(len(ctxHandles) - 1)
}

//export SessionUIStreamStart
func SessionUIStreamStart(ctxHandle C.int, format *C.char, tsDiscon C.int, bitrate C.int) C.int {
	err = appflinger.SessionUIStreamStart(ctxHandles[ctxHandle], C.GoString(format),
		GoBool(tsDiscon), int(bitrate))
	if err != nil {
		log.Println(err)
		return -1
	}
	return 0
}

//export SessionGetSessionId
func SessionGetSessionId(ctxHandle C.int) *C.char {
	var sessionId string
	sessionId, err = appflinger.SessionGetSessionId(ctxHandles[ctxHandle])
	if err != nil {
		log.Println(err)
		return nil
	}

	return C.CString(sessionId) // Needs to be freed by caller
}

//export SessionGetSessionContext
func SessionGetSessionContext(sessionId *C.char) C.int {
	var ctx *appflinger.SessionContext
	ctx, err = appflinger.SessionGetSessionContext(C.GoString(sessionId))
	if err != nil {
		log.Println(err)
		return -1
	}
	for idx, val := range ctxHandles {
		if val == ctx {
			return C.int(idx)
		}
	}
	return -1
}

//export SessionGetUIURL
func SessionGetUIURL(ctxHandle C.int, format *C.char, tsDiscon C.int, bitrate C.int) *C.char {
	var uri string
	uri, err = appflinger.SessionGetUIURL(ctxHandles[ctxHandle], C.GoString(format), GoBool(tsDiscon), int(bitrate))
	if err != nil {
		log.Println(err)
		return nil
	}

	return C.CString(uri) // Needs to be freed by caller
}

//export SessionSendEvent
func SessionSendEvent(ctxHandle C.int, eventType *C.char, code C.int, ch C.int, x C.int, y C.int) C.int {
	err = appflinger.SessionSendEvent(ctxHandles[ctxHandle], C.GoString(eventType), int(code), rune(ch), int(x), int(y))
	if err != nil {
		log.Println(err)
		return -1
	}
	return 0
}

//export SessionSendNotificationVideoStateChange
func SessionSendNotificationVideoStateChange(ctxHandle C.int, instanceId *C.char, readyState C.int,
	networkState C.int, paused C.int, seeking C.int, duration C.double, time C.double, videoWidth C.int, videoHeight C.int) C.int {
	err = appflinger.SessionSendNotificationVideoStateChange(ctxHandles[ctxHandle], C.GoString(instanceId),
		int(readyState), int(networkState), GoBool(paused), GoBool(seeking), float64(duration), float64(time),
		int(videoWidth), int(videoHeight))
	if err != nil {
		log.Println(err)
		return -1
	}
	return 0
}

//export GetErr
func GetErr() *C.char {
	return C.CString(err.Error())
}

func main() {}
