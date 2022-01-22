// Copyright 2015 TVersity Inc. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	// #include <stdio.h>
	// #include <stdlib.h>
	// #include "callbacks.h"
	"C"

	appflinger "github.com/tversity/appflinger-go"
)
import (
	"fmt"
	"unsafe"
)

const ()

// This struct will implement the appflinger.AppFlinger interface which is needed in order to
// receive the control channel commands and process them
type AppflingerListener struct {
	// C callback pointers
	// Note - we cannot invoke C function pointers from Go so we use a helper C function to do it
	// e.g. to invoke the on_ui_frame_cb function pointer we use C.invoke_on_ui_frame()
	cb *C.appflinger_callbacks_t
}

func NewAppflingerListener(cb *C.appflinger_callbacks_t) (self *AppflingerListener) {
	self = &AppflingerListener{}
	self.cb = cb
	return
}

func getCPointer(memSize int) unsafe.Pointer {
	return C.malloc(C.size_t(memSize))
}

type cDataToFloatSliceConverter interface {
	// When implementing the interface in both functions (getCPointerToDoubleArray, convertCPointerToFloatSlice) 
	// we must use the same constant for memory size to be allocated and used in conversion.
	// Conversion should use data copying to the Go slice. 
	// Freeing unsafe.Pointer returned by getCPointerToDoubleArray is a caller code responsibility.
	getCPointerToDoubleArray() unsafe.Pointer
	convertCPointerToFloatSlice(cPointer unsafe.Pointer, len int) (result []float64)
}

// GoBufferedResult implements cDataToFloatSliceConverter interface 
// to serve conversion of appflinger.GetBufferedResult fields
type GoBufferedResult struct{}

func (r GoBufferedResult) getCPointerToDoubleArray() unsafe.Pointer {
	return getCPointer(C.MSE_BUFFERED_LENGTH * C.sizeof_double)
}

func (r GoBufferedResult) convertCPointerToFloatSlice(cPointer unsafe.Pointer, len int) (result []float64) {
	v := (*[C.MSE_BUFFERED_LENGTH]float64)(cPointer)	
	result = make([]float64, len)
	copy(result, (*v)[0:len])
	return
}

// Implementation of appflinger.AppFlinger interface that just delegates to C Callbacks

func (self *AppflingerListener) Load(sessionId string, instanceId string, url string) (err error) {
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	cUrl := C.CString(url)
	rc := C.invoke_load(self.cb.load_cb, cSessionId, cInstanceId, cUrl)
	if rc != 0 {
		err = fmt.Errorf("Failed to load media")
	} else {
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	C.free(unsafe.Pointer(cUrl))
	return
}

func (self *AppflingerListener) CancelLoad(sessionId string, instanceId string) (err error) {
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	rc := C.invoke_cancel_load(self.cb.cancel_load_cb, cSessionId, cInstanceId)
	if rc != 0 {
		err = fmt.Errorf("Failed to cancel load of media")
	} else {
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	return
}

func (self *AppflingerListener) Pause(sessionId string, instanceId string) (err error) {
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	rc := C.invoke_pause(self.cb.pause_cb, cSessionId, cInstanceId)
	if rc != 0 {
		err = fmt.Errorf("Failed to pause media")
	} else {
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	return
}

func (self *AppflingerListener) Play(sessionId string, instanceId string) (err error) {
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	rc := C.invoke_play(self.cb.play_cb, cSessionId, cInstanceId)
	if rc != 0 {
		err = fmt.Errorf("Failed to play media")
	} else {
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	return
}

func (self *AppflingerListener) Seek(sessionId string, instanceId string, time float64) (err error) {
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	rc := C.invoke_seek(self.cb.seek_cb, cSessionId, cInstanceId, C.double(time))
	if rc != 0 {
		err = fmt.Errorf("Failed to seek media")
	} else {
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	return
}

func (self *AppflingerListener) GetPaused(sessionId string, instanceId string) (paused bool, err error) {
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	var cPaused C.int
	rc := C.invoke_get_paused(self.cb.get_paused_cb, cSessionId, cInstanceId, &cPaused)
	if rc != 0 {
		err = fmt.Errorf("Failed to get pause state")
	} else {
		paused = GoBool(cPaused)
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	return
}

func (self *AppflingerListener) GetSeeking(sessionId string, instanceId string) (seeking bool, err error) {
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	var cSeeking C.int
	rc := C.invoke_get_seeking(self.cb.get_seeking_cb, cSessionId, cInstanceId, &cSeeking)
	if rc != 0 {
		err = fmt.Errorf("Failed to get seeking state")
	} else {
		seeking = GoBool(cSeeking)
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	return
}

func (self *AppflingerListener) GetDuration(sessionId string, instanceId string) (duration float64, err error) {
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	var cDuration C.double
	rc := C.invoke_get_duration(self.cb.get_duration_cb, cSessionId, cInstanceId, &cDuration)
	if rc != 0 {
		err = fmt.Errorf("Failed to get duration of media")
	} else {
		duration = float64(cDuration)
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	return
}

func (self *AppflingerListener) GetCurrentTime(sessionId string, instanceId string) (time float64, err error) {
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	var cTime C.double
	rc := C.invoke_get_current_time(self.cb.get_current_time_cb, cSessionId, cInstanceId, &cTime)
	if rc != 0 {
		err = fmt.Errorf("Failed to get current time of media")
	} else {
		time = float64(cTime)
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	return
}

func (self *AppflingerListener) GetNetworkState(sessionId string, instanceId string) (networkState int, err error) {
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	var cNetworkState C.int
	rc := C.invoke_get_network_state(self.cb.get_network_state_cb, cSessionId, cInstanceId, &cNetworkState)
	if rc != 0 {
		err = fmt.Errorf("Failed to get network state")
	} else {
		networkState = int(cNetworkState)
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	return
}

func (self *AppflingerListener) GetReadyState(sessionId string, instanceId string) (readyState int, err error) {
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	var cReadyState C.int
	rc := C.invoke_get_ready_state(self.cb.get_ready_state_cb, cSessionId, cInstanceId, &cReadyState)
	if rc != 0 {
		err = fmt.Errorf("Failed to get ready state")
	} else {
		readyState = int(cReadyState)
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	return
}

func (self *AppflingerListener) GetSeekable(sessionId string, instanceId string, result *appflinger.GetSeekableResult) (err error) {
	var duration float64
	duration, err = self.GetDuration(sessionId, instanceId)
	if err != nil {
		return
	}

	result.Start = []float64{0}
	result.End = []float64{duration}
	err = nil
	return
}

func (self *AppflingerListener) GetBuffered(sessionId string, instanceId string, result *appflinger.GetBufferedResult) (err error) {
	var duration float64
	duration, err = self.GetDuration(sessionId, instanceId)
	if err != nil {
		return
	}

	result.Start = []float64{0}
	result.End = []float64{duration}
	err = nil
	return
}

func (self *AppflingerListener) SetRect(sessionId string, instanceId string, x int, y int, width int, height int) (err error) {
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	rc := C.invoke_set_rect(self.cb.set_rect_cb, cSessionId, cInstanceId, C.int(x), C.int(y), C.int(width), C.int(height))
	if rc != 0 {
		err = fmt.Errorf("Failed to set media display rectangle")
	} else {
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	return
}

func (self *AppflingerListener) SetVisible(sessionId string, instanceId string, visible bool) (err error) {
	err = nil
	return
}

func (self *AppflingerListener) SetRate(sessionId string, instanceId string, rate float64) (err error) {
	err = nil
	return
}

func (self *AppflingerListener) SetVolume(sessionId string, instanceId string, volume float64) (err error) {
	err = nil
	return
}

func (self *AppflingerListener) AddSourceBuffer(sessionId string, instanceId string, sourceId string, mimeType string) (err error) {
	if self.cb == nil || self.cb.add_source_buffer_cb == nil {
		return
	}
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	cSourceId := C.CString(sourceId)
	cMimeType := C.CString(mimeType)
	rc := C.invoke_add_source_buffer(self.cb.add_source_buffer_cb, cSessionId, cInstanceId, cSourceId, cMimeType);
	if rc != 0 {
		err = fmt.Errorf("Failed to add source buffer")
	} else {
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	C.free(unsafe.Pointer(cSourceId))
	C.free(unsafe.Pointer(cMimeType))
	return
}

func (self *AppflingerListener) RemoveSourceBuffer(sessionId string, instanceId string, sourceId string) (err error) {
	if self.cb == nil || self.cb.remove_source_buffer_cb == nil {
		return
	}
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	cSourceId := C.CString(sourceId)
	rc := C.invoke_remove_source_buffer(self.cb.remove_source_buffer_cb, cSessionId, cInstanceId, cSourceId);
	if rc != 0 {
		err = fmt.Errorf("Failed to remove source buffer")
	} else {
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	C.free(unsafe.Pointer(cSourceId))
	return
}

func (self *AppflingerListener) AbortSourceBuffer(sessionId string, instanceId string, sourceId string) (err error) {
	if self.cb == nil || self.cb.abort_source_buffer_cb == nil {
		return
	}
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	cSourceId := C.CString(sourceId)
	rc := C.invoke_abort_source_buffer(self.cb.abort_source_buffer_cb, cSessionId, cInstanceId, cSourceId);
	if rc != 0 {
		err = fmt.Errorf("Failed to abort source buffer")
	} else {
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	C.free(unsafe.Pointer(cSourceId))
	return
}

func (self *AppflingerListener) AppendBuffer(sessionId string, instanceId string, sourceId string, appendWindowStart float64, appendWindowEnd float64,
	bufferId string, bufferOffset int, bufferLength int, payload []byte, result *appflinger.GetBufferedResult) (err error) {
	if self.cb == nil || self.cb.append_buffer_cb == nil {
		return
	}
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	cSourceId := C.CString(sourceId)
	cBufferId := C.CString(bufferId)
	cPayload := C.CBytes(payload)

	var r GoBufferedResult
	cBufferedStart := r.getCPointerToDoubleArray()
	cBufferedEnd := r.getCPointerToDoubleArray()
	var cBufferedLength C.int;
	
	rc := C.invoke_append_buffer(self.cb.append_buffer_cb, cSessionId, cInstanceId, cSourceId, C.double(appendWindowStart), C.double(appendWindowEnd),
		cBufferId, C.int(bufferOffset), C.int(bufferLength), cPayload, C.uint(len(payload)), cBufferedStart, cBufferedEnd, &cBufferedLength);
	if rc != 0 {
		err = fmt.Errorf("Failed to append buffer")
	} else {
		err = nil
	}

	result.Start = r.convertCPointerToFloatSlice(cBufferedStart, int(cBufferedLength))
	result.End = r.convertCPointerToFloatSlice(cBufferedEnd, int(cBufferedLength))
	
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	C.free(unsafe.Pointer(cSourceId))
	C.free(unsafe.Pointer(cBufferId))
	C.free(unsafe.Pointer(cPayload))
	C.free(unsafe.Pointer(cBufferedStart))
	C.free(unsafe.Pointer(cBufferedEnd))
	return
}

func (self *AppflingerListener) SetAppendMode(sessionId string, instanceId string, sourceId string, mode int) (err error) {
	if self.cb == nil || self.cb.set_append_mode_cb == nil {
		return
	}
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	cSourceId := C.CString(sourceId)
	rc := C.invoke_set_append_mode(self.cb.set_append_mode_cb, cSessionId, cInstanceId, cSourceId, C.int(mode));
	if rc != 0 {
		err = fmt.Errorf("Failed to set append mode")
	} else {
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	C.free(unsafe.Pointer(cSourceId))
	return
}

func (self *AppflingerListener) SetAppendTimestampOffset(sessionId string, instanceId string, sourceId string, timestampOffset float64) (err error) {
	if self.cb == nil || self.cb.set_append_timestamp_offset_cb == nil {
		return
	}
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	cSourceId := C.CString(sourceId)
	rc := C.invoke_set_append_timestamp_offset(self.cb.set_append_timestamp_offset_cb, cSessionId, cInstanceId, cSourceId, C.double(timestampOffset));
	if rc != 0 {
		err = fmt.Errorf("Failed to set append mode")
	} else {
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	C.free(unsafe.Pointer(cSourceId))
	return
}

func (self *AppflingerListener) RemoveBufferRange(sessionId string, instanceId string, sourceId string, start float64, end float64) (err error) {
	if self.cb == nil || self.cb.remove_buffer_range_cb == nil {
		return
	}
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	cSourceId := C.CString(sourceId)
	rc := C.invoke_remove_buffer_range(self.cb.remove_buffer_range_cb, cSessionId, cInstanceId, cSourceId, C.double(start), C.double(end));
	if rc != 0 {
		err = fmt.Errorf("Failed to set append mode")
	} else {
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	C.free(unsafe.Pointer(cSourceId))
	return
}

func (self *AppflingerListener) ChangeSourceBufferType(sessionId string, instanceId string, sourceId string, mimeType string) (err error) {
	if self.cb == nil || self.cb.change_source_buffer_type_cb == nil {
		return
	}
	cSessionId := C.CString(sessionId)
	cInstanceId := C.CString(instanceId)
	cSourceId := C.CString(sourceId)
	cMimeType := C.CString(mimeType)
	rc := C.invoke_change_source_buffer_type(self.cb.change_source_buffer_type_cb, cSessionId, cInstanceId, cSourceId, cMimeType);
	if rc != 0 {
		err = fmt.Errorf("Failed to set append mode")
	} else {
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cInstanceId))
	C.free(unsafe.Pointer(cSourceId))
	C.free(unsafe.Pointer(cMimeType))
	return
}

func (self *AppflingerListener) LoadResource(sessionId string, url string, method string, headers string, resourceId string,
	byteRangeStart int, byteRangeEnd int, sequenceNumber int, payload []byte, result *appflinger.LoadResourceResult) (err error) {
	err = nil
	result.Code = "404"
	result.Headers = ""
	result.BufferId = ""
	result.BufferLength = 0
	result.Payload = nil
	return
}

func (self *AppflingerListener) DeleteResource(sessionId string, BufferId string) (err error) {
	err = nil
	return
}

func (self *AppflingerListener) RequestKeySystem(sessionId string, keySystem string, supportedConfigurations []appflinger.EMEMediaKeySystemConfiguration, result *appflinger.RequestKeySystemResult) (err error) {
	err = nil
	return
}

func (self *AppflingerListener) CdmCreate(sessionId string, keySystem string, securityOrigin string, allowDistinctiveIdentifier bool, allowPersistentState bool) (cdmId string, err error) {
	err = nil
	return
}

func (self *AppflingerListener) CdmSetServerCertificate(sessionId string, cdmId string, payload []byte) (err error) {
	err = nil
	return
}

func (self *AppflingerListener) CdmSessionCreate(sessionId string, eventInstanceId string, cdmId string, sessionType string, initDataType string, payload []byte) (cdmSessionId string, expiration float64, err error) {
	err = nil
	return
}

func (self *AppflingerListener) CdmSessionUpdate(sessionId string, eventInstanceId string, cdmId string, cdmSessionId string, payload []byte) (err error) {
	err = nil
	return
}

func (self *AppflingerListener) CdmSessionLoad(sessionId string, eventInstanceId string, cdmId string, cdmSessionId string) (loaded bool, expiration float64, err error) {
	err = nil
	return
}

func (self *AppflingerListener) CdmSessionRemove(sessionId string, eventInstanceId string, cdmId string, cdmSessionId string) (err error) {
	err = nil
	return
}

func (self *AppflingerListener) CdmSessionClose(sessionId string, eventInstanceId string, cdmId string, cdmSessionId string) (err error) {
	err = nil
	return
}

func (self *AppflingerListener) SetCdm(sessionId string, instanceId string, cdmId string) (err error) {
	err = nil
	return
}

func (self *AppflingerListener) SendMessage(sessionId string, message string) (result string, err error) {
	err = nil
	result = ""
	return
}

func (self *AppflingerListener) OnPageLoad(sessionId string) (err error) {
	err = nil
	return
}

func (self *AppflingerListener) OnAddressBarChanged(sessionId string, url string) (err error) {
	err = nil
	return
}

func (self *AppflingerListener) OnTitleChanged(sessionId string, title string) (err error) {
	err = nil
	return
}

func (self *AppflingerListener) OnPageClose(sessionId string) (err error) {
	err = nil
	return
}

func (self *AppflingerListener) OnUIFrame(sessionId string, isCodecConfig bool, isKeyFrame bool, idx int, pts int, dts int, data []byte) (err error) {
	if self.cb == nil || self.cb.on_ui_frame_cb == nil {
		return
	}
	cSessionId := C.CString(sessionId)
	cData := C.CBytes(data)
	rc := C.invoke_on_ui_frame(self.cb.on_ui_frame_cb, cSessionId, CBool(isCodecConfig), CBool(isKeyFrame), C.int(idx), C.longlong(pts),
		C.longlong(dts), cData, C.uint(len(data)))
	if rc != 0 {
		err = fmt.Errorf("Failed to process frame")
	} else {
		err = nil
	}
	C.free(unsafe.Pointer(cSessionId))
	C.free(unsafe.Pointer(cData))
	return
}
