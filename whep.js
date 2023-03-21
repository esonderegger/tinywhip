async function negotiateConnectionWithClientOffer(peerConnection, endpoint) {
  /** https://developer.mozilla.org/en-US/docs/Web/API/RTCPeerConnection/createOffer */
  const offer = await peerConnection.createOffer();
  /** https://developer.mozilla.org/en-US/docs/Web/API/RTCPeerConnection/setLocalDescription */
  await peerConnection.setLocalDescription(offer);
  /** Wait for ICE gathering to complete */
  let ofr = await waitToCompleteICEGathering(peerConnection);
  if (!ofr) {
    throw Error("failed to gather ICE candidates for offer");
  }
  /**
   * As long as the connection is open, attempt to...
   */
  while (peerConnection.connectionState !== "closed") {
    /**
     * This response contains the server's SDP offer.
     * This specifies how the client should communicate,
     * and what kind of media client and server have negotiated to exchange.
     */
    let response = await postSDPOffer(endpoint, ofr.sdp);
    if (response.status === 201 || response.status === 200) {
      let answerSDP = await response.text();
      await peerConnection.setRemoteDescription(
        new RTCSessionDescription({ type: "answer", sdp: answerSDP })
      );
      return response.headers.get("Location");
    } else if (response.status === 405) {
      console.error("Update the URL passed into the WHIP or WHEP client");
    } else {
      const errorMessage = await response.text();
      console.error(errorMessage);
    }
    /** Limit reconnection attempts to at-most once every 5 seconds */
    await new Promise((r) => setTimeout(r, 5000));
  }
}

async function postSDPOffer(endpoint, data) {
  return await fetch(endpoint, {
    method: "POST",
    mode: "cors",
    headers: {
      "content-type": "application/sdp",
    },
    body: data,
  });
}

async function waitToCompleteICEGathering(peerConnection) {
  return new Promise((resolve) => {
    /** Wait at most 1 second for ICE gathering. */
    setTimeout(function () {
      resolve(peerConnection.localDescription);
    }, 1000);
    peerConnection.onicegatheringstatechange = (ev) =>
      peerConnection.iceGatheringState === "complete" &&
      resolve(peerConnection.localDescription);
  });
}

class WHEPClient {
  constructor(endpoint, videoElement) {
    this.endpoint = endpoint;
    this.videoElement = videoElement;
    this.stream = new MediaStream();
    /**
     * Create a new WebRTC connection, using public STUN servers with ICE,
     * allowing the client to disover its own IP address.
     * https://developer.mozilla.org/en-US/docs/Web/API/WebRTC_API/Protocols#ice
     */
    this.peerConnection = new RTCPeerConnection({
      iceServers: [
        // {
        // 	urls: 'stun:stun.cloudflare.com:3478',
        // },
      ],
      bundlePolicy: "max-bundle",
    });
    /** https://developer.mozilla.org/en-US/docs/Web/API/RTCPeerConnection/addTransceiver */
    this.peerConnection.addTransceiver("video", {
      direction: "recvonly",
    });
    this.peerConnection.addTransceiver("audio", {
      direction: "recvonly",
    });
    /**
     * When new tracks are received in the connection, store local references,
     * so that they can be added to a MediaStream, and to the <video> element.
     *
     * https://developer.mozilla.org/en-US/docs/Web/API/RTCPeerConnection/track_event
     */
    this.peerConnection.ontrack = (event) => {
      const track = event.track;
      const currentTracks = this.stream.getTracks();
      const streamAlreadyHasVideoTrack = currentTracks.some(
        (track) => track.kind === "video"
      );
      const streamAlreadyHasAudioTrack = currentTracks.some(
        (track) => track.kind === "audio"
      );
      switch (track.kind) {
        case "video":
          if (streamAlreadyHasVideoTrack) {
            break;
          }
          this.stream.addTrack(track);
          break;
        case "audio":
          if (streamAlreadyHasAudioTrack) {
            break;
          }
          this.stream.addTrack(track);
          break;
        default:
          console.log("got unknown track " + track);
      }
    };
    this.peerConnection.addEventListener("connectionstatechange", (ev) => {
      if (this.peerConnection.connectionState !== "connected") {
        return;
      }
      if (!this.videoElement.srcObject) {
        this.videoElement.srcObject = this.stream;
      }
    });
    this.peerConnection.addEventListener("negotiationneeded", (ev) => {
      negotiateConnectionWithClientOffer(this.peerConnection, this.endpoint);
    });
  }
}

const videoEl = document.getElementById("remote-video");
const muteToggleButton = document.getElementById("mute-toggle");

const clientWhep = new WHEPClient(
  `http://localhost:8080${window.location.pathname}`,
  videoEl
);

function toggleMute() {
  if (videoEl.muted) {
    videoEl.muted = false;
    muteToggleButton.textContent = "Mute";
  } else {
    videoEl.muted = true;
    muteToggleButton.textContent = "Un-mute";
  }
}

muteToggleButton.addEventListener("click", toggleMute, false);
