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

func (self *AppflingerListenerStub) Load(instanceId string, url string) (err error) {
	err = nil
	self.loaded = true
	self.paused = true
	return
}

func (self *AppflingerListenerStub) CancelLoad(instanceId string) (err error) {
	err = nil
	self.loaded = false
	self.paused = true
	return
}

func (self *AppflingerListenerStub) Pause(instanceId string) (err error) {
	if self.loaded {
		err = nil
		self.paused = true
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) Play(instanceId string) (err error) {
	if self.loaded {
		err = nil
		self.paused = false
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) Seek(instanceId string, time float64) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) GetPaused(instanceId string) (paused bool, err error) {
	if self.loaded {
		err = nil
		paused = self.paused
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) GetSeeking(instanceId string) (seeking bool, err error) {
	if self.loaded {
		err = nil
		seeking = false
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) GetDuration(instanceId string) (duration float64, err error) {
	if self.loaded {
		err = nil
		duration = MockDuration
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) GetCurrentTime(instanceId string) (time float64, err error) {
	if self.loaded {
		err = nil
		time = 0
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) GetNetworkState(instanceId string) (networkState int, err error) {
	if self.loaded {
		err = nil
		networkState = appflinger.NETWORK_STATE_LOADED
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) GetReadyState(instanceId string) (readyState int, err error) {
	if self.loaded {
		err = nil
		readyState = appflinger.READY_STATE_HAVE_ENOUGH_DATA
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) GetMaxTimeSeekable(instanceId string) (maxTimeSeekable float64, err error) {
	if self.loaded {
		err = nil
		maxTimeSeekable = 0
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) GetBuffered(instanceId string, result *appflinger.GetBufferedResult) (err error) {
	if self.loaded {
		err = nil
		result.Start = []float64{0}
		result.End = []float64{MockDuration}
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) SetRect(instanceId string, x int, y int, width int, height int) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) SetVisible(instanceId string, visible bool) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) AddSourceBuffer(instanceId string, sourceId string, mimeType string) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) RemoveSourceBuffer(instanceId string, sourceId string) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) ResetSourceBuffer(instanceId string, sourceId string) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) AppendBuffer(instanceId string, sourceId string, appendWindowStart float64, appendWindowEnd float64,
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

func (self *AppflingerListenerStub) LoadResource(url string, method string, headers string, resourceId string,
	byteRangeStart int, byteRangeEnd int, sequenceNumber int, payload []byte, result *appflinger.LoadResourceResult) (err error) {
	if self.loaded {
		err = nil
		result.Code = "404"
		result.Headers = ""
		result.BufferId = ""
		result.BufferLength = 0
		result.Payload = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppflingerListenerStub) SendMessage(message string) (result string, err error) {
	err = nil
	result = ""
	return
}

func (self *AppflingerListenerStub) OnPageLoad() (err error) {
	err = nil
	return
}

func (self *AppflingerListenerStub) OnAddressBarChanged(url string) (err error) {
	err = nil
	return
}

func (self *AppflingerListenerStub) OnTitleChanged(title string) (err error) {
	err = nil
	return
}

func (self *AppflingerListenerStub) OnPageClose() (err error) {
	err = nil
	return
}

func (self *AppflingerListenerStub) OnUIFrame(isCodecConfig bool, isKeyFrame bool, idx int, pts int, dts int, data []byte) (err error) {
	err = nil
	return
}
