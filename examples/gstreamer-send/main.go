package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"

	"github.com/pions/webrtc"

	gst "github.com/pions/webrtc/examples/internal/gstreamer-src"
	"github.com/pions/webrtc/examples/internal/signal"
)

const web = `<!DOCTYPE html>
<html>
<head>
	<title>WebRTC Demo</title>
	<style> 
		textarea {
			width: 500px;
			min-height: 75px;
		}
	</style>
</head>

<body>
<button onclick="window.startSession()"> Start Session </button><br />
<br />
Video<br />
<div id="remoteVideos"></div> <br />

Logs<br />
<div id="div"></div>

<script type="text/javascript">
/* eslint-env browser */

let pc = new RTCPeerConnection({
	iceServers: [
    	{
      		urls: 'stun:stun.l.google.com:19302'
    	}
  	]
})

let log = msg => {
	document.getElementById('div').innerHTML += msg + '<br>'
}

window.startSession = () => {
	pc.ontrack = function (event) {
		var el = document.createElement(event.track.kind)
		el.srcObject = event.streams[0]
		el.autoplay = true
		el.controls = true
	  
		document.getElementById('remoteVideos').appendChild(el)
	}
	  
	pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)
	pc.onicecandidate = event => {
		if (event.candidate === null) {
			let sdp = btoa(JSON.stringify(pc.localDescription))

			fetch("/webrtc", { headers: { "x-webrtc-offer": sdp }})
			.then(res => res.text())
			.then(sd => {
				try {
					pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(sd))))
				} catch (e) {
					alert(e)
				}
			})
			.catch(err => {
				console.log("u")
				alert("sorry, there are no results for your search")
			});
		}
	}
	
	pc.createOffer({ offerToReceiveVideo: true, offerToReceiveAudio: true }).then(d => pc.setLocalDescription(d)).catch(log)
}
</script>
</body>
</html>`

func main() {
	audioSrc := flag.String("audio-src", "audiotestsrc", "GStreamer audio src")
	videoSrc := flag.String("video-src", "videotestsrc", "GStreamer video src")
	flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, web)
	})

	http.HandleFunc("/webrtc", func(w http.ResponseWriter, r *http.Request) {
		sdp := r.Header.Get("x-webrtc-offer")
		if sdp == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "missing header: x-webrtc-offer")
			return
		}
		fmt.Fprintf(w, startStream(audioSrc, videoSrc, sdp))
	})

	fmt.Println("Listening on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func startStream(audioSrc, videoSrc *string, offerB64 string) string {

	// Everything below is the pion-WebRTC API! Thanks for using it ❤️.

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	// Create a audio track
	opusTrack, err := peerConnection.NewTrack(webrtc.DefaultPayloadTypeOpus, rand.Uint32(), "audio", "pion1")
	if err != nil {
		panic(err)
	}
	_, err = peerConnection.AddTrack(opusTrack)
	if err != nil {
		panic(err)
	}

	// Create a video track
	vp8Track, err := peerConnection.NewTrack(webrtc.DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion2")
	if err != nil {
		panic(err)
	}
	_, err = peerConnection.AddTrack(vp8Track)
	if err != nil {
		panic(err)
	}

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	signal.Decode(offerB64, &offer)

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	// Create an answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	// Output the answer in base64 so we can paste it in browser
	fmt.Println(signal.Encode(answer))

	// Start pushing buffers on these tracks
	gst.CreatePipeline(webrtc.Opus, opusTrack, *audioSrc).Start()
	gst.CreatePipeline(webrtc.VP8, vp8Track, *videoSrc).Start()

	return signal.Encode(answer)
}
