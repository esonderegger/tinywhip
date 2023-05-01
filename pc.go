package main

import (
	"fmt"
	"os"

	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v3"
)

type VideoCodecSalient struct {
	MimeType       string
	SDPFmtpLine    string
	PayloadType    webrtc.PayloadType
	RtxPayloadType webrtc.PayloadType
}

func createPeerConnection(codecs []codecInfo, config webrtc.Configuration) (*webrtc.PeerConnection, error) {
	m := &webrtc.MediaEngine{}
	for _, codec := range codecs {
		if err := m.RegisterCodec(codec.codecParameters, codec.codecType); err != nil {
			return nil, err
		}
	}
	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		return nil, err
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))
	pc, err := api.NewPeerConnection(config)
	return pc, err
}

func codecsFromStream(stream []streamTrack) []codecInfo {
	codecs := []codecInfo{}
	for _, track := range stream {
		codecs = append(codecs, track.codec)
	}
	return codecs
}

func supportedCodecs() []codecInfo {
	codecs := []codecInfo{}
	_, supportOpus := os.LookupEnv("SUPPORT_OPUS")
	if supportOpus {
		codecs = append(codecs, codecInfo{
			codecType: webrtc.RTPCodecTypeAudio,
			codecParameters: webrtc.RTPCodecParameters{
				RTPCodecCapability: webrtc.RTPCodecCapability{
					MimeType:     webrtc.MimeTypeOpus,
					ClockRate:    48000,
					Channels:     2,
					SDPFmtpLine:  "minptime=10;useinbandfec=1",
					RTCPFeedback: nil},
				PayloadType: 111,
			},
		})
	}
	_, supportVP8 := os.LookupEnv("SUPPORT_VP8")
	if supportVP8 {
		vp8Codecs := videoCodecInfo([]VideoCodecSalient{
			{
				MimeType:       webrtc.MimeTypeVP8,
				SDPFmtpLine:    "",
				PayloadType:    96,
				RtxPayloadType: 97,
			},
		})
		codecs = append(codecs, vp8Codecs...)
	}
	_, supportVP9 := os.LookupEnv("SUPPORT_VP9")
	if supportVP9 {
		vp9Codecs := videoCodecInfo([]VideoCodecSalient{
			{
				MimeType:       webrtc.MimeTypeVP9,
				SDPFmtpLine:    "profile-id=0",
				PayloadType:    98,
				RtxPayloadType: 99,
			},
			{
				MimeType:       webrtc.MimeTypeVP9,
				SDPFmtpLine:    "profile-id=1",
				PayloadType:    100,
				RtxPayloadType: 101,
			},
		})
		codecs = append(codecs, vp9Codecs...)
	}
	_, supportH264 := os.LookupEnv("SUPPORT_H264")
	if supportH264 {
		h264Codecs := videoCodecInfo([]VideoCodecSalient{
			{
				MimeType:       webrtc.MimeTypeH264,
				SDPFmtpLine:    "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f",
				PayloadType:    102,
				RtxPayloadType: 121,
			},
			{
				MimeType:       webrtc.MimeTypeH264,
				SDPFmtpLine:    "level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42001f",
				PayloadType:    127,
				RtxPayloadType: 120,
			},
			{
				MimeType:       webrtc.MimeTypeH264,
				SDPFmtpLine:    "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
				PayloadType:    125,
				RtxPayloadType: 107,
			},
			{
				MimeType:       webrtc.MimeTypeH264,
				SDPFmtpLine:    "level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42e01f",
				PayloadType:    108,
				RtxPayloadType: 109,
			},
			{
				MimeType:       webrtc.MimeTypeH264,
				SDPFmtpLine:    "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=640032",
				PayloadType:    123,
				RtxPayloadType: 118,
			},
		})
		codecs = append(codecs, h264Codecs...)
	}
	return codecs
}

func videoCodecInfo(salients []VideoCodecSalient) []codecInfo {
	videoFeedback := []webrtc.RTCPFeedback{
		{
			Type:      "goog-remb",
			Parameter: "",
		},
		{
			Type:      "ccm",
			Parameter: "fir",
		},
		{
			Type:      "nack",
			Parameter: "",
		},
		{
			Type:      "nack",
			Parameter: "pli",
		},
	}
	codecs := []codecInfo{}
	for _, salient := range salients {
		codecs = append(codecs, codecInfo{
			codecType: webrtc.RTPCodecTypeVideo,
			codecParameters: webrtc.RTPCodecParameters{
				RTPCodecCapability: webrtc.RTPCodecCapability{
					MimeType:     salient.MimeType,
					ClockRate:    90000,
					Channels:     0,
					SDPFmtpLine:  salient.SDPFmtpLine,
					RTCPFeedback: videoFeedback,
				},
				PayloadType: salient.PayloadType,
			},
		})
		codecs = append(codecs, codecInfo{
			codecType: webrtc.RTPCodecTypeVideo,
			codecParameters: webrtc.RTPCodecParameters{
				RTPCodecCapability: webrtc.RTPCodecCapability{
					MimeType:     "video/rtx",
					ClockRate:    90000,
					Channels:     0,
					SDPFmtpLine:  fmt.Sprintf("apt=%d", salient.PayloadType),
					RTCPFeedback: nil,
				},
				PayloadType: salient.RtxPayloadType,
			},
		})
	}
	return codecs
}

// func createPeerConnectionOld() (*webrtc.PeerConnection, error) {
// 	m := &webrtc.MediaEngine{}
// 	for _, codec := range []webrtc.RTPCodecParameters{
// 		{
// 			RTPCodecCapability: webrtc.RTPCodecCapability{webrtc.MimeTypeOpus, 48000, 2, "minptime=10;useinbandfec=1", nil},
// 			PayloadType:        111,
// 		},
// 	} {
// 		if err := m.RegisterCodec(codec, webrtc.RTPCodecTypeAudio); err != nil {
// 			panic(err)
// 		}
// 	}
// 	videoRTCPFeedback := []webrtc.RTCPFeedback{{"goog-remb", ""}, {"ccm", "fir"}, {"nack", ""}, {"nack", "pli"}}
// 	for _, codec := range []webrtc.RTPCodecParameters{
// 		{
// 			RTPCodecCapability: webrtc.RTPCodecCapability{webrtc.MimeTypeVP9, 90000, 0, "profile-id=0", videoRTCPFeedback},
// 			PayloadType:        98,
// 		},
// 		{
// 			RTPCodecCapability: webrtc.RTPCodecCapability{"video/rtx", 90000, 0, "apt=98", nil},
// 			PayloadType:        99,
// 		},
// 		{
// 			RTPCodecCapability: webrtc.RTPCodecCapability{webrtc.MimeTypeVP9, 90000, 0, "profile-id=1", videoRTCPFeedback},
// 			PayloadType:        100,
// 		},
// 		{
// 			RTPCodecCapability: webrtc.RTPCodecCapability{"video/rtx", 90000, 0, "apt=100", nil},
// 			PayloadType:        101,
// 		},
// 		// Baseline level 3.1
// 		{
// 			RTPCodecCapability: webrtc.RTPCodecCapability{webrtc.MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f", videoRTCPFeedback},
// 			PayloadType:        102,
// 		},
// 		{
// 			RTPCodecCapability: webrtc.RTPCodecCapability{"video/rtx", 90000, 0, "apt=102", nil},
// 			PayloadType:        121,
// 		},

// 		// Baseline level 3.1
// 		{
// 			RTPCodecCapability: webrtc.RTPCodecCapability{webrtc.MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42001f", videoRTCPFeedback},
// 			PayloadType:        127,
// 		},
// 		{
// 			RTPCodecCapability: webrtc.RTPCodecCapability{"video/rtx", 90000, 0, "apt=127", nil},
// 			PayloadType:        120,
// 		},

// 		{
// 			RTPCodecCapability: webrtc.RTPCodecCapability{webrtc.MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f", videoRTCPFeedback},
// 			PayloadType:        125,
// 		},
// 		{
// 			RTPCodecCapability: webrtc.RTPCodecCapability{"video/rtx", 90000, 0, "apt=125", nil},
// 			PayloadType:        107,
// 		},
// 		{
// 			RTPCodecCapability: webrtc.RTPCodecCapability{webrtc.MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42e01f", videoRTCPFeedback},
// 			PayloadType:        108,
// 		},
// 		{
// 			RTPCodecCapability: webrtc.RTPCodecCapability{"video/rtx", 90000, 0, "apt=108", nil},
// 			PayloadType:        109,
// 		},
// 		// Baseline level 3.1
// 		{
// 			RTPCodecCapability: webrtc.RTPCodecCapability{webrtc.MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42001f", videoRTCPFeedback},
// 			PayloadType:        127,
// 		},
// 		{
// 			RTPCodecCapability: webrtc.RTPCodecCapability{"video/rtx", 90000, 0, "apt=127", nil},
// 			PayloadType:        120,
// 		},
// 		// High Level 5
// 		{
// 			RTPCodecCapability: webrtc.RTPCodecCapability{webrtc.MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=640032", videoRTCPFeedback},
// 			PayloadType:        123,
// 		},
// 		{
// 			RTPCodecCapability: webrtc.RTPCodecCapability{"video/rtx", 90000, 0, "apt=123", nil},
// 			PayloadType:        118,
// 		},
// 	} {
// 		if err := m.RegisterCodec(codec, webrtc.RTPCodecTypeVideo); err != nil {
// 			panic(err)
// 		}
// 	}
// 	i := &interceptor.Registry{}
// 	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
// 		return nil, err
// 	}
// 	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))

// 	pc, err := api.NewPeerConnection(*pcConfig)
// 	return pc, err
// }
