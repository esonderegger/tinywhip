const startButton = document.getElementById("start");
const monitorElement = document.getElementById("monitor");

const pc = new RTCPeerConnection({
  iceServers: [],
});

async function startStream(localSdp) {
  const { pathname } = window.location;
  const response = await fetch(pathname, {
    method: "POST",
    headers: {
      "Content-Type": "application/sdp",
    },
    body: localSdp,
  });
  const remoteSdp = await response.text();
  try {
    await pc.setRemoteDescription({ type: "answer", sdp: remoteSdp });
  } catch (e) {
    alert(e);
  }
}

pc.ontrack = function (event) {
  console.log("ontrack", event);
};

pc.oniceconnectionstatechange = (e) => {
  console.log("oniceconnectionstatechange", pc.iceConnectionState, e);
};

pc.onicecandidate = (event) => {
  console.log("onicecandidate", event);
};

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
  const gdmOptions = {
    video: true,
    audio: true,
  };
  const stream = await startCapture(gdmOptions);
  monitorElement.srcObject = stream;
  monitorElement.play();
  stream.getTracks().forEach((t) => {
    pc.addTrack(t, stream);
  });
  pc.addTransceiver("audio", { direction: "sendonly" });
  pc.addTransceiver("video", { direction: "sendonly" });
  const lD = await pc.createOffer();
  await pc.setLocalDescription(lD);
  await startStream(pc.localDescription.sdp);
}

startButton.addEventListener("click", startSending, false);
