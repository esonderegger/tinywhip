
const startButton = document.getElementById("start");

const pc = new RTCPeerConnection({
  iceServers: [],
});

async function startStream(localSdp) {
  // const { pathname } = window.location;
  const response = await fetch("/", {
    method: "POST",
    headers: {
      "Content-Type": "application/sdp",
    },
    body: localSdp,
  });
  const remoteSdp = await response.text();
  try {
    // const remoteDescription = { type: "answer", sdp: remoteSdp };
    // pc.setRemoteDescription(new RTCSessionDescription(remoteDescription));
    await pc.setRemoteDescription({ type:"answer",sdp: remoteSdp });
    console.log("remote description set");
  } catch (e) {
    alert(e);
  }
}

pc.ontrack = function (event) {
  console.log("on track??", event);
  // if (event.track.kind === "video") {
  //   videoEl.srcObject = event.streams[0];
  // }
};

pc.oniceconnectionstatechange = (e) => {
  console.log("oniceconnectionstatechange", pc.iceConnectionState, e);
};

pc.onicecandidate = (event) => {
  console.log("onicecandidate", event);
  // if (event.candidate === null) {
  //   const { sdp } = pc.localDescription;
  //   startStream(sdp);
  // }
};

// Offer to receive both audio and video
// pc.addTransceiver("audio", { direction: "recvonly" });
// pc.addTransceiver("video", { direction: "recvonly" });
// pc.createOffer()
//   .then((d) => pc.setLocalDescription(d))
//   .catch((e) => console.log(e));

async function startCapture(displayMediaOptions) {
  let captureStream = null;

  try {
    captureStream = await navigator.mediaDevices.getDisplayMedia(
      displayMediaOptions
    );
  } catch (err) {
    console.error(`Error getting display media: ${err}`);
  }
  return captureStream;
}

async function startSending() {
  console.log("this is where it starts");
  const gdmOptions = {
    video: true,
    audio: true,
  };
  const stream = await startCapture(gdmOptions);
  stream.getTracks().forEach((t) => {
    console.log("track", t)
    // pc.addTrack(t, stream);
    pc.addTrack(t);
  });
  // const whip = new WHIPClient();
  // whip.publish(pc, "http://localhost:8080/", "testing");
  const lD = pc.createOffer({
    offerToReceiveAudio: !1,
    offerToReceiveVideo: !1,
  });
  await pc.setLocalDescription(lD);
  console.log(pc.localDescription.sdp);
  await startStream(pc.localDescription.sdp);
}

startButton.addEventListener("click", startSending, false);
