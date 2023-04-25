package main

import (
	_ "embed"

	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

type HtmlData struct {
	Css string
	Js  string
}

//go:embed tinywhip.css
var tinywhipCss string

//go:embed whip.html.tmpl
var whipHtml string

//go:embed whep.html.tmpl
var whepHtml string

//go:embed whip.js
var whipJs string

//go:embed whep.js
var whepJs string

var pcConfig *webrtc.Configuration
var pcs map[string]*webrtc.PeerConnection
var streams map[string][]*webrtc.TrackLocalStaticRTP

func getPortFromEnv(key string, fallback int) int {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	intVar, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return intVar
}

// Read incoming RTCP packets
// Before these packets are returned they are processed by interceptors. For things
// like NACK this needs to be called.
func readRtcpFromSender(sender *webrtc.RTPSender) {
	rtcpBuf := make([]byte, 1500)
	for {
		if _, _, rtcpErr := sender.Read(rtcpBuf); rtcpErr != nil {
			return
		}
	}
}

func getWhipStream(w http.ResponseWriter, req *http.Request) {
	hd := HtmlData{tinywhipCss, whipJs}
	t := template.Must(template.New("index").Parse(whipHtml))
	err := t.Execute(w, hd)
	if err != nil {
		panic(err)
	}
}

func getWhepStream(w http.ResponseWriter, req *http.Request) {
	hd := HtmlData{tinywhipCss, whepJs}
	t := template.Must(template.New("index").Parse(whepHtml))
	err := t.Execute(w, hd)
	if err != nil {
		panic(err)
	}
}

func postWhipStream(w http.ResponseWriter, req *http.Request) {
	id := req.URL.Path[len("/whip/"):]
	fmt.Printf("Got WHIP request: %s\n", id)

	// If previous WHIP sender failed to send DELETE, clean up now.
	delete(streams, id)
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "Problem reading WHIP request body", http.StatusBadRequest)
		return
	}

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  string(reqBody),
	}

	pc, err := webrtc.NewPeerConnection(*pcConfig)
	if err != nil {
		panic(err)
	}
	pc.OnTrack(func(tr *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		fmt.Printf("Got track: %s\n", tr.StreamID())
		fmt.Printf("Codec info: %s\n", tr.Codec().MimeType)

		tl, err := webrtc.NewTrackLocalStaticRTP(tr.Codec().RTPCodecCapability, tr.ID(), tr.StreamID())
		if err != nil {
			fmt.Printf("Failed to create track: %s\n", err)
			return
		}
		stream, ok := streams[id]
		if ok {
			stream = append(stream, tl)
		} else {
			streams[id] = []*webrtc.TrackLocalStaticRTP{tl}
		}
		go func() {
			ticker := time.NewTicker(time.Second * 2)
			for range ticker.C {
				errSend := pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(tr.SSRC())}})
				if errSend != nil {
					fmt.Println(errSend)
				}
			}
		}()

		for {
			p, _, err := tr.ReadRTP()
			if err != nil {
				fmt.Printf("Failed to read RTP: %s\n", err)
				return
			}
			if err := tl.WriteRTP(p); err != nil {
				fmt.Printf("Failed to write RTP: %s\n", err)
				return
			}
		}
	})
	if err = pc.SetRemoteDescription(offer); err != nil {
		panic(err)
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err = pc.SetLocalDescription(answer); err != nil {
		panic(err)
	}
	<-gatherComplete
	w.Header().Set("Content-Type", "application/sdp")
	fmt.Fprintf(w, pc.LocalDescription().SDP)
}

func postWhepStream(w http.ResponseWriter, req *http.Request) {
	id := req.URL.Path[len("/whep/"):]
	fmt.Printf("Got WHEP request: %s\n", id)
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "Problem reading WHEP request body", http.StatusBadRequest)
		return
	}

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  string(reqBody),
	}

	pc, err := webrtc.NewPeerConnection(*pcConfig)
	if err != nil {
		panic(err)
	}

	stream, ok := streams[id]
	if ok {
		for _, t := range stream {
			fmt.Printf("Adding track: %s\n", t.StreamID())

			if _, err := pc.AddTransceiverFromTrack(t, webrtc.RtpTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly}); err != nil {
				fmt.Printf("Failed to add track: %s\n", err)
				return
			}
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	pc.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())

		if connectionState == webrtc.ICEConnectionStateFailed {
			if closeErr := pc.Close(); closeErr != nil {
				panic(closeErr)
			}
		}
	})
	if err = pc.SetRemoteDescription(offer); err != nil {
		panic(err)
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err = pc.SetLocalDescription(answer); err != nil {
		panic(err)
	}
	<-gatherComplete
	w.Header().Set("Content-Type", "application/sdp")
	fmt.Fprintf(w, pc.LocalDescription().SDP)
}

func handleWhip(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		getWhipStream(w, req)
		return
	case "POST":
		postWhipStream(w, req)
		return
	default:
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}
}

func handleWhep(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		getWhepStream(w, req)
		return
	case "POST":
		postWhepStream(w, req)
		return
	default:
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}
}

func router(w http.ResponseWriter, req *http.Request) {
	if strings.HasPrefix(req.URL.Path, "/whip/") {
		handleWhip(w, req)
		return
	}
	if strings.HasPrefix(req.URL.Path, "/whep/") {
		handleWhep(w, req)
		return
	}
	http.Error(w, "404 not found.", http.StatusNotFound)
	return
}

func main() {
	pcConfigAlt := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
	}
	pcConfig = &pcConfigAlt
	pcs = make(map[string]*webrtc.PeerConnection)
	streams = make(map[string][]*webrtc.TrackLocalStaticRTP)
	http.HandleFunc("/", router)
	httpPort := getPortFromEnv("WHEP_PORT", 8080)
	port := ":" + strconv.Itoa(httpPort)
	fmt.Fprintf(os.Stdout, "Serving on http://localhost%s\n", port)
	http.ListenAndServe(port, nil)
}
