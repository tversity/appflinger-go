appflinger-go
=============

This is the official Go client SDK for AppFlinger (www.appflinger.com). It does not come with a full client implementation but rather it is just the client SDK along with some examples.

A full reference documentation of the SDK is available at:
https://godoc.org/github.com/tversity/appflinger-go

There is also a C client SDK (available in binary form), a [Javascript client SDK](https://github.com/ronenmiz/appflinger-js), and a [C# client SDK](https://github.com/ronenmiz/appflinger-mediaroom), as well as a full client implementation for the Raspberry Pi (please contact TVersity for more information). 

The server side code is closed source and is available for licensing from TVersity.

### What is AppFlinger?

AppFlinger (www.appflinger.com) is a [cloud browser](https://www.w3.org/TR/cloud-browser-arch/), i.e. it is an HTML5 browser (based on Chromium) running in the cloud and delivered to client devices as a video or image stream. AppFlinger is a server side technology that can be deployed in the cloud or on premise and be utilized for delivering a consistent HTML5 content experience to any device, in a hardware agnostic manner. AppFlinger physically [isolates](https://en.wikipedia.org/wiki/Browser_isolation) edge devices and edge networks from the content they are rendering, thus enabling next generation security while delivering on the promise of any content on any device.

AppFlinger makes it possible to run HTML5 content on any device and in the same time makes the full power of desktop-grade HTML5 browsers available to content publishers.

### AppFlinger Uniqueness

AppFlinger is unique in its ability to deliver desktop grade HTML5 browser experience with just having a solid video player on the client and very low bandwidth and CPU requirement on the server.
