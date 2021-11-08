// Copyright 2015 TVersity Inc. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	"errors"

	"github.com/tversity/appflinger-go"
)

const (
	MockDuration = 60
)

// This struct will implement the appflinger.AppFlinger interface which is needed in order to
// receive the control channel commands and process them
type AppflingerListenerStub struct {
	loaded bool
	paused bool
}

func NewAppflingerListenerStub() (self *AppflingerListenerStub) {
	self = &AppflingerListenerStub{}
	self.loaded = false
	self.paused = true
	return
}

// Stub implementation of all the methods in  appflinger.AppFlinger interface
// A full client should replace the stub with proper implementation

func (self *AppflingerListenerStub) Load(sessionId string, instanceId string, url string) (err error) {
	err = nil
	self.loaded = true
	self.paused = true
	return
}

func (self *AppflingerListenerStub) CancelLoad(sessionId string, instanceId string) (err error) {
	err = nil
	self.loaded = false
	self.paused = true
	return
}

func (self *AppflingerListenerStub) Pause(sessionId string, instanceId string) (err error) {
	if self.loaded {
		err = nil
		self.paused = true
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) Play(sessionId string, instanceId string) (err error) {
	if self.loaded {
		err = nil
		self.paused = false
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) Seek(sessionId string, instanceId string, time float64) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) GetPaused(sessionId string, instanceId string) (paused bool, err error) {
	if self.loaded {
		err = nil
		paused = self.paused
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) GetSeeking(sessionId string, instanceId string) (seeking bool, err error) {
	if self.loaded {
		err = nil
		seeking = false
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) GetDuration(sessionId string, instanceId string) (duration float64, err error) {
	if self.loaded {
		err = nil
		duration = MockDuration
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) GetCurrentTime(sessionId string, instanceId string) (time float64, err error) {
	if self.loaded {
		err = nil
		time = 0
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) GetNetworkState(sessionId string, instanceId string) (networkState int, err error) {
	if self.loaded {
		err = nil
		networkState = appflinger.NETWORK_STATE_LOADED
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) GetReadyState(sessionId string, instanceId string) (readyState int, err error) {
	if self.loaded {
		err = nil
		readyState = appflinger.READY_STATE_HAVE_ENOUGH_DATA
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) GetSeekable(sessionId string, instanceId string, result *appflinger.GetSeekableResult) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) GetBuffered(sessionId string, instanceId string, result *appflinger.GetBufferedResult) (err error) {
	if self.loaded {
		err = nil
		result.Start = []float64{0}
		result.End = []float64{MockDuration}
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) SetRect(sessionId string, instanceId string, x int, y int, width int, height int) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) SetVisible(sessionId string, instanceId string, visible bool) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListener) SetRate(sessionId string, instanceId string, rate float64) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListener) SetVolume(sessionId string, instanceId string, volume float64) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) AddSourceBuffer(sessionId string, instanceId string, sourceId string, mimeType string) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) RemoveSourceBuffer(sessionId string, instanceId string, sourceId string) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) AbortSourceBuffer(sessionId string, instanceId string, sourceId string) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) AppendBuffer(sessionId string, instanceId string, sourceId string, appendWindowStart float64, appendWindowEnd float64,
	bufferId string, bufferOffset int, bufferLength int, payload []byte, result *appflinger.GetBufferedResult) (err error) {
	if self.loaded {
		result.Start = nil
		result.End = nil
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) SetAppendMode(sessionId string, instanceId string, sourceId string, mode int) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) SetAppendTimestampOffset(sessionId string, instanceId string, sourceId string, timestampOffset float64) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) RemoveBufferRange(sessionId string, instanceId string, sourceId string, start float64, end float64) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) ChangeSourceBufferType(sessionId string, instanceId string, sourceId string, mimeType string) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) LoadResource(sessionId string, url string, method string, headers string, resourceId string,
	byteRangeStart int, byteRangeEnd int, sequenceNumber int, payload []byte, result *appflinger.LoadResourceResult) (err error) {
	err = nil
	result.Code = "404"
	result.Headers = ""
	result.BufferId = ""
	result.BufferLength = 0
	result.Payload = nil
	return
}

func (self *AppflingerListenerStub) DeleteResource(sessionId string, BufferId string) (err error) {
	err = nil
	return
}

func (self *AppflingerListenerStub) RequestKeySystem(sessionId string, keySystem string, supportedConfigurations []appflinger.EMEMediaKeySystemConfiguration, result *appflinger.RequestKeySystemResult) (err error) {
	err = nil
	return
}

func (self *AppflingerListenerStub) CdmCreate(sessionId string, keySystem string, securityOrigin string, allowDistinctiveIdentifier bool, allowPersistentState bool) (cdmId string, err error) {
	err = nil
	return
}

func (self *AppflingerListenerStub) CdmSetServerCertificate(sessionId string, cdmId string, payload []byte) (err error) {
	err = nil
	return
}

func (self *AppflingerListenerStub) CdmSessionCreate(sessionId string, eventInstanceId string, cdmId string, sessionType string, initDataType string, payload []byte) (cdmSessionId string, expiration float64, err error) {
	err = nil
	return
}

func (self *AppflingerListenerStub) CdmSessionUpdate(sessionId string, eventInstanceId string, cdmId string, cdmSessionId string, payload []byte) (err error) {
	err = nil
	return
}

func (self *AppflingerListenerStub) CdmSessionLoad(sessionId string, eventInstanceId string, cdmId string, cdmSessionId string) (loaded bool, expiration float64, err error) {
	err = nil
	return
}

func (self *AppflingerListenerStub) CdmSessionRemove(sessionId string, eventInstanceId string, cdmId string, cdmSessionId string) (err error) {
	err = nil
	return
}

func (self *AppflingerListenerStub) CdmSessionClose(sessionId string, eventInstanceId string, cdmId string, cdmSessionId string) (err error) {
	err = nil
	return
}

func (self *AppflingerListenerStub) SetCdm(sessionId string, instanceId string, cdmId string) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) SendMessage(sessionId string, message string) (result string, err error) {
	err = nil
	result = ""
	return
}

func (self *AppflingerListenerStub) OnPageLoad(sessionId string) (err error) {
	err = nil
	return
}

func (self *AppflingerListenerStub) OnAddressBarChanged(sessionId string, url string) (err error) {
	err = nil
	return
}

func (self *AppflingerListenerStub) OnTitleChanged(sessionId string, title string) (err error) {
	err = nil
	return
}

func (self *AppflingerListenerStub) OnPageClose(sessionId string) (err error) {
	err = nil
	return
}

func (self *AppflingerListenerStub) OnUIVideoFrame(sessionId string, isCodecConfig bool, isKeyFrame bool, idx int, pts int, dts int, data []byte) (err error) {
	err = nil
	return
}
