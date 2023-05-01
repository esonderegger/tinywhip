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

type pcInitError struct {
	message       string
	httpError     int
	originalError error
}

type codecInfo struct {
	codecType       webrtc.RTPCodecType
	codecParameters webrtc.RTPCodecParameters
}

type streamTrack struct {
	codec    codecInfo
	localRTP *webrtc.TrackLocalStaticRTP
}

var pcConfig *webrtc.Configuration
var pcs map[string]*webrtc.PeerConnection
var streams map[string][]streamTrack

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

func readRtcpFromSender(sender *webrtc.RTPSender) {
	rtcpBuf := make([]byte, 1500)
	for {
		// n, a, rtcpErr := sender.Read(rtcpBuf)
		_, attrs, rtcpErr := sender.Read(rtcpBuf)
		if rtcpErr != nil {
			fmt.Printf("Error reading incoming RTCP: %s\n", rtcpErr)
			return
		}
		// fmt.Printf("First RTCP thing: %d\n", n)
		if 2+2 == 5 {
			fmt.Printf("RTCP attributes: %s\n", attrs)
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

func initPeerConnection(w http.ResponseWriter, req *http.Request, codecs []codecInfo) (*webrtc.PeerConnection, error) {
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "Problem reading HTTP request body", http.StatusBadRequest)
		return nil, err
	}
	// fmt.Printf("Got SDP: %s\n", reqBody)

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  string(reqBody),
	}
	// unmarshalled, err := offer.Unmarshal()
	// if err != nil {
	// 	http.Error(w, "Problem unmarshalling SDP", http.StatusBadRequest)
	// 	return nil, err
	// }
	// attrs := unmarshalled.Attributes
	// fmt.Println("Looping over media descriptions")
	// for _, attr := range attrs {
	// 	if strings.HasPrefix(attr.String(), "rtpmap:") {
	// 		fmt.Printf("Value: %s\n", attr.Value)
	// 	}
	// }

	// pc, err := webrtc.NewPeerConnection(*pcConfig)
	pc, err := createPeerConnection(codecs, *pcConfig)
	if err != nil {
		http.Error(w, "Problem creating peer connection", http.StatusInternalServerError)
		return nil, err
	}
	if err = pc.SetRemoteDescription(offer); err != nil {
		http.Error(w, "Problem setting remote description", http.StatusInternalServerError)
		return nil, err
	}
	return pc, nil
}

func postWhipStream(w http.ResponseWriter, req *http.Request) {
	id := req.URL.Path[len("/whip/"):]
	fmt.Printf("Got WHIP request: %s\n", id)

	// If previous WHIP sender failed to send DELETE, clean up now.
	// delete(streams, id)
	streams[id] = []streamTrack{}
	pc, err := initPeerConnection(w, req, supportedCodecs())
	if err != nil {
		return
	}
	pc.OnTrack(func(tr *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		fmt.Printf("Got track: %s\n", tr.ID())
		fmt.Printf("Codec info: %s\n", tr.Codec().MimeType)
		fmt.Printf("Codec info: %s\n", tr.Kind())

		tl, err := webrtc.NewTrackLocalStaticRTP(tr.Codec().RTPCodecCapability, tr.ID(), tr.StreamID())
		if err != nil {
			fmt.Printf("Failed to create track: %s\n", err)
			return
		}
		newTrack := streamTrack{
			codec:    codecInfo{codecType: tr.Kind(), codecParameters: tr.Codec()},
			localRTP: tl,
		}
		_, ok := streams[id]
		if ok {
			streams[id] = append(streams[id], newTrack)
		} else {
			panic("race condition when appending tracks to stream")
		}
		go func() {
			ticker := time.NewTicker(time.Second * 3)
			for range ticker.C {
				trSSRC := uint32(tr.SSRC())
				packets := []rtcp.Packet{}
				packets = append(packets, &rtcp.PictureLossIndication{
					MediaSSRC: trSSRC,
				})
				packets = append(packets, &rtcp.ReceiverEstimatedMaximumBitrate{
					Bitrate: 2000000.0,
					SSRCs:   []uint32{trSSRC},
				})
				errSend := pc.WriteRTCP(packets)
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
	stream, ok := streams[id]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	pc, err := initPeerConnection(w, req, codecsFromStream(stream))
	if err != nil {
		return
	}

	fmt.Printf("Tracks in stream: %d\n", len(stream))
	for _, t := range stream {
		fmt.Printf("Adding track: %s\n", t.localRTP.ID())
		sender, err := pc.AddTrack(t.localRTP)
		// transceiver, err := pc.AddTransceiverFromTrack(t, webrtc.RtpTransceiverInit{
		// 	Direction: webrtc.RTPTransceiverDirectionSendonly,
		// })
		if err != nil {
			fmt.Printf("Failed to add track: %s\n", err)
			return
		}
		go readRtcpFromSender(sender)
		// go readRtcpFromSender(transceiver.Sender())
	}

	pc.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())

		if connectionState == webrtc.ICEConnectionStateFailed {
			if closeErr := pc.Close(); closeErr != nil {
				panic(closeErr)
			}
		}
	})

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}
	if err = pc.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
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
	pcConfig = &webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
	}
	pcs = make(map[string]*webrtc.PeerConnection)
	streams = make(map[string][]streamTrack)
	http.HandleFunc("/", router)
	httpPort := getPortFromEnv("HTTP_PORT", 8080)
	port := ":" + strconv.Itoa(httpPort)
	fmt.Fprintf(os.Stdout, "Serving on http://localhost%s\n", port)
	http.ListenAndServe(port, nil)
}
