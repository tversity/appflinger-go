appflinger-go
=============

This is the official Go client SDK for AppFlinger (www.appflinger.com). It does not come with a full client implementation but rather it is just the client SDK along with some examples.

There is also a C client SDK (available in binary form), a Javascript client SDK, and a C# client SDK, as well as a full client implementation for the Raspberry Pi (please contact TVersity to gain access to it). 

The server side code is closed source and is available for licensing from TVersity, please contact us for more information.

### What is AppFlinger?

AppFlinger (www.appflinger.com) is an HTML5 browser (based on Chromium) running in the cloud and delivered to client devices as a video stream,. It is also a full solution for aggregation, monetization and delivery of TV apps (based on HTML5) to set-top boxes and smart TVs. AppFlinger utilizes the cloud for running the HTML5 TV apps and delivers them as a video stream to target devices with unprecedented quality and responsiveness.

AppFlinger makes it possible to run HTML5 TV apps on any device and in the same time makes the full power of desktop-grade HTML5 browsers available to TV app developers.

AppFlinger is available out of the box with many premium TV apps.

If you are interested in AppFlinger, please contact us.

If you are the developer of a TV App in HTML5 and would like to reach the existing deployment base of AppFlinger, please contact us as well.

### AppFlinger Uniqueness

AppFlinger is unique in its ability to deliver desktop grade HTML5 browser experience with just having a solid video player on the client and very low bandwidth and CPU requirement on the server (this is unlike the typical cloud gaming scenario where CPU and bandwidth requirements on the server are virtually cost-prohibitive).

This is achieved by breaking the browser experience to two video streams. The UI video stream is created in real-time by the server (by encoding the content of the browser window) and delivered to the client for low-latency rendering. The other video stream is the actual video played via the HTML5 media element (aka HTML5 video tag).

This approach allows HTML5 TV apps, like the one at www.youtube.com/tv, to run in the cloud and be delivered to any client device (including ultra low-end and legacy devices).


## What is a Cloud Browser?
A cloud browser runs on a server and therefore does all the rendering and
execution of web content on the server. The client tends to be a very
lightweight implementation of a remote UI protocol of some sort. the W3C has a
cloud browser task force with additional information[here](https://www.w3.org/2011/webtv/wiki/Main_Page/Cloud_Browser_TF).
