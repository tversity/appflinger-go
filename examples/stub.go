// Copyright 2015 TVersity Inc. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"github.com/tversity/appflinger-go"
)

// This struct will implement the appflinger.AppFlinger interface which is needed in order to
// receive the control channel commands and process them
type AppFlingerStub struct {
	loaded bool
	paused bool
}

func NewAppFlingerStub() (self *AppFlingerStub) {
	self = &AppFlingerStub{}
	self.loaded = false
	self.paused = true
	return
}

// Stub implementation of all the methods in  appflinger.AppFlinger interface
// A full client should replace the stub with proper implementation

func (self *AppFlingerStub) Load(url string) (err error) {
	err = nil
	self.loaded = true
	self.paused = true
	return
}

func (self *AppFlingerStub) CancelLoad() (err error) {
	err = nil
	self.loaded = false
	self.paused = true
	return
}

func (self *AppFlingerStub) Pause() (err error) {
	if self.loaded {
		err = nil
		self.paused = true
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppFlingerStub) Play() (err error) {
	if self.loaded {
		err = nil
		self.paused = false
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppFlingerStub) Seek(time float64) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppFlingerStub) GetPaused() (paused bool, err error) {
	if self.loaded {
		err = nil
		paused = self.paused
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppFlingerStub) GetSeeking() (seeking bool, err error) {
	if self.loaded {
		err = nil
		seeking = false
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppFlingerStub) GetDuration() (duration float64, err error) {
	if self.loaded {
		err = nil
		duration = 60 // some stub value
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppFlingerStub) GetCurrentTime() (time float64, err error) {
	if self.loaded {
		err = nil
		time = 0
	} else {
		err = errors.New("No video loaded")
	}	
	return
}

func (self *AppFlingerStub) GetNetworkState() (networkState int, err error) {
	if self.loaded {
		err = nil
		networkState = appflinger.NETWORK_STATE_LOADED
	} else {
		err = errors.New("No video loaded")
	}
	return
}

func (self *AppFlingerStub) GetReadyState() (readyState int, err error) {
	if self.loaded {
		err = nil
		readyState = appflinger.READY_STATE_HAVE_ENOUGH_DATA
	} else {
		err = errors.New("No video loaded")
	}	
	return
}

func (self *AppFlingerStub) GetMaxTimeSeekable() (maxTimeSeekable float64, err error) {
	if self.loaded {
		err = nil
		maxTimeSeekable = 0
	} else {
		err = errors.New("No video loaded")
	}	
	return
}

func (self *AppFlingerStub) SetRect(x uint, y uint, width uint, height uint) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}	
	return
}

func (self *AppFlingerStub) SetVisible(visible bool) (err error) {
	if self.loaded {
		err = nil
	} else {
		err = errors.New("No video loaded")
	}	
	return
}

func (self *AppFlingerStub) SendMessage(message string) (result string, err error) {
	err = nil
	result = ""
	return
}

func (self *AppFlingerStub) OnPageLoad() (err error) {
	err = nil
	return
}

func (self *AppFlingerStub) OnAddressBarChanged(url string) (err error) {
	err = nil
	return
}

func (self *AppFlingerStub) OnTitleChanged(title string) (err error) {
	err = nil
	return
}

func (self *AppFlingerStub) OnPageClose() (err error) {
	err = nil
	return
}
