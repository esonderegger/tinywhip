import { EventEmitter as e } from "events";
import { write as t, parse as i } from "sdp-transform";
class s {
  sendOffer(e, t, i) {
    return fetch(e, {
      method: "POST",
      headers: { "Content-Type": "application/sdp", Authorization: t },
      body: i,
    });
  }
  getConfiguration(e, t) {
    return fetch(e, { method: "OPTIONS", headers: { Authorization: t } });
  }
  delete(e) {
    return fetch(e, { method: "DELETE" });
  }
  updateIce(e, t, i) {
    return fetch(e, {
      method: "PATCH",
      headers: { "Content-Type": "application/trickle-ice-sdpfrag", ETag: t },
      body: i,
    });
  }
}
class o extends e {
  constructor({
    endpoint: e,
    opts: t,
    whipProtocol: i,
    peerConnectionFactory: o,
  }) {
    super(),
      (this.whipEndpoint = void 0),
      (this.opts = void 0),
      (this.peer = void 0),
      (this.resource = void 0),
      (this.eTag = void 0),
      (this.extensions = void 0),
      (this.resourceResolve = void 0),
      (this.iceCredentials = void 0),
      (this.mediaMids = []),
      (this.whipProtocol = void 0),
      (this.peerConnectionFactory = void 0),
      (this.iceGatheringTimeout = void 0),
      (this.waitingForCandidates = !1),
      (this.whipEndpoint = new URL(e)),
      (this.opts = t),
      (this.opts.noTrickleIce = !!t.noTrickleIce),
      (this.whipProtocol = i || new s()),
      (this.peerConnectionFactory = o || ((e) => new RTCPeerConnection(e))),
      this.initPeer();
  }
  initPeer() {
    (this.peer = this.peerConnectionFactory({
      iceServers: this.opts.iceServers || [
        { urls: ["stun:stun.l.google.com:19302"] },
      ],
    })),
      this.peer.addEventListener(
        "iceconnectionstatechange",
        this.onIceConnectionStateChange.bind(this)
      ),
      this.peer.addEventListener(
        "icecandidateerror",
        this.onIceCandidateError.bind(this)
      ),
      this.peer.addEventListener(
        "connectionstatechange",
        this.onConnectionStateChange.bind(this)
      ),
      this.peer.addEventListener(
        "icecandidate",
        this.onIceCandidate.bind(this)
      ),
      this.peer.addEventListener(
        "onicegatheringstatechange",
        this.onIceGatheringStateChange.bind(this)
      );
  }
  log(...e) {
    this.opts.debug && console.log("WHIPClient", ...e);
  }
  error(...e) {
    console.error("WHIPClient", ...e);
  }
  makeSDPTransformCandidate(e) {
    var t;
    return {
      foundation: e.foundation,
      component: "rtp" === e.component ? 0 : 1,
      transport: e.protocol.toString(),
      priority: e.priority,
      ip: e.address,
      port: e.port,
      type: e.type.toString(),
      raddr: null == e ? void 0 : e.relatedAddress,
      rport: null == e ? void 0 : e.relatedPort,
      tcptype: null == e || null == (t = e.tcpType) ? void 0 : t.toString(),
    };
  }
  makeTrickleIceSdpFragment(e) {
    if (!this.iceCredentials || 0 === this.mediaMids.length)
      return void this.error(
        "Missing local SDP meta data, cannot send trickle ICE candidate"
      );
    let i = {
      media: [],
      iceUfrag: this.iceCredentials.ufrag,
      icePwd: this.iceCredentials.pwd,
    };
    for (let t of this.mediaMids) {
      const s = {
        type: "audio",
        port: 9,
        protocol: "RTP/AVP",
        payloads: "0",
        rtp: [],
        fmtp: [],
        mid: t,
        candidates: [this.makeSDPTransformCandidate(e)],
      };
      i.media.push(s);
    }
    return t(i).replace("v=0\r\ns= \r\n", "");
  }
  async onIceCandidate(e) {
    if ("icecandidate" !== e.type) return;
    const t = e.candidate;
    if (t)
      if (this.supportTrickleIce()) {
        const e = this.makeTrickleIceSdpFragment(t),
          i = await this.getResourceUrl();
        (await this.whipProtocol.updateIce(i, this.eTag, e)).ok ||
          (this.log("Trickle ICE not supported by endpoint"),
          (this.opts.noTrickleIce = !0));
      } else this.log(t.candidate);
  }
  async onConnectionStateChange(e) {
    this.log("PeerConnectionState", this.peer.connectionState),
      "failed" === this.peer.connectionState && (await this.destroy());
  }
  onIceConnectionStateChange(e) {
    this.log("IceConnectionState", this.peer.iceConnectionState);
  }
  onIceCandidateError(e) {
    this.log("IceCandidateError", e);
  }
  onIceGatheringStateChange(e) {
    "complete" === this.peer.iceGatheringState &&
      !this.supportTrickleIce() &&
      this.waitingForCandidates &&
      this.onDoneWaitingForCandidates();
  }
  getICEConnectionState() {
    return this.peer.iceConnectionState;
  }
  async startSdpExchange() {
    const e = await this.peer.createOffer({
        offerToReceiveAudio: !1,
        offerToReceiveVideo: !1,
      }),
      t = e.sdp && i(e.sdp);
    if (!t) return Promise.reject();
    t.iceUfrag && t.icePwd
      ? (this.iceCredentials = { pwd: t.icePwd, ufrag: t.iceUfrag })
      : 0 !== t.media.length &&
        t.media[0].iceUfrag &&
        t.media[0].icePwd &&
        (this.iceCredentials = {
          pwd: t.media[0].icePwd,
          ufrag: t.media[0].iceUfrag,
        });
    for (let e of t.media) e.mid && this.mediaMids.push(e.mid);
    var s;
    await this.peer.setLocalDescription(e),
      this.supportTrickleIce()
        ? await this.sendOffer()
        : ((this.waitingForCandidates = !0),
          (this.iceGatheringTimeout = setTimeout(
            this.onIceGatheringTimeout.bind(this),
            (null == (s = this.opts) ? void 0 : s.timeout) || 2e3
          )));
  }
  onIceGatheringTimeout() {
    this.log("onIceGatheringTimeout"),
      !this.supportTrickleIce() &&
        this.waitingForCandidates &&
        this.onDoneWaitingForCandidates();
  }
  async onDoneWaitingForCandidates() {
    (this.waitingForCandidates = !1),
      clearTimeout(this.iceGatheringTimeout),
      await this.sendOffer();
  }
  async sendOffer() {
    this.log("Sending offer"), this.log(this.peer.localDescription.sdp);
    const e = await this.whipProtocol.sendOffer(
      this.whipEndpoint.toString(),
      this.opts.authkey,
      this.peer.localDescription.sdp
    );
    if (e.ok) {
      (this.resource = e.headers.get("Location")),
        this.resource.match(/^http/) ||
          (this.resource = new URL(
            this.resource,
            this.whipEndpoint
          ).toString()),
        this.log("WHIP Resource", this.resource),
        (this.eTag = e.headers.get("ETag")),
        this.log("eTag", this.eTag);
      const t = e.headers.get("Link");
      (this.extensions = t ? t.split(",").map((e) => e.trimStart()) : []),
        this.log("WHIP Resource Extensions", this.extensions),
        this.resourceResolve &&
          (this.resourceResolve(this.resource), (this.resourceResolve = null));
      const i = await e.text();
      await this.peer.setRemoteDescription({ type: "answer", sdp: i });
    } else
      this.error(
        "IceCandidate",
        "Failed to setup stream connection with endpoint",
        e.status,
        await e.text()
      );
  }
  async doFetchICEFromEndpoint() {
    let e = [];
    const t = await this.whipProtocol.getConfiguration(
      this.whipEndpoint.toString(),
      this.opts.authkey
    );
    return (
      t.ok &&
        t.headers.forEach((t, i) => {
          if ("link" == i) {
            const i = (function (e) {
              let t;
              if (e.match(/rel="ice-server"/))
                if (e.match(/^stun:/)) {
                  const [i, s] = e.match(/^(stun:\S+);/);
                  s && (t = { urls: s });
                } else
                  e.match(/^turn:/) &&
                    e.split(";").forEach((e) => {
                      if (e.match(/^turn:/)) {
                        const [i, s] = e.match(/^(turn:\S+)/);
                        t = { urls: s };
                      } else if (e.match(/^\s*username[=:]/)) {
                        const [i, s] = e.match(/^\s*username[=:]\s*"*([^"]+)/);
                        t.username = s;
                      } else if (e.match(/^\s*credential[=:]/)) {
                        const [i, s] = e.match(
                          /^\s*credential[=:]\s*"*([^"]+)/
                        );
                        t.credential = s;
                      }
                    });
              return t;
            })(t);
            i && e.push(i);
          }
        }),
      e
    );
  }
  supportTrickleIce() {
    return !this.opts.noTrickleIce;
  }
  async setIceServersFromEndpoint() {
    if (this.opts.authkey) {
      this.log("Fetching ICE config from endpoint");
      const e = await this.doFetchICEFromEndpoint();
      this.peer.setConfiguration({ iceServers: e });
    } else
      this.error(
        "No authkey is provided so cannot fetch ICE config from endpoint."
      );
  }
  async ingest(e) {
    if (
      (this.peer || this.initPeer(),
      e.getTracks().forEach((t) => this.peer.addTrack(t, e)),
      !this.opts.noTrickleIce)
    ) {
      const e = await this.whipProtocol.getConfiguration(
        this.whipEndpoint.toString(),
        this.opts.authkey
      );
      let t = !1;
      e.headers.get("access-control-allow-methods") &&
        (t = e.headers
          .get("access-control-allow-methods")
          .split(",")
          .map((e) => e.trim())
          .includes("PATCH")),
        t
          ? ((this.opts.noTrickleIce = !1),
            this.log(
              "Endpoint says it supports Trickle ICE as PATCH is an allowed method"
            ))
          : ((this.opts.noTrickleIce = !0),
            this.log("Endpoint does not support Trickle ICE"));
    }
    await this.startSdpExchange();
  }
  async destroy() {
    const e = await this.getResourceUrl();
    await this.whipProtocol.delete(e).catch((e) => this.error("destroy()", e));
    const t = this.peer.getSenders();
    t &&
      t.forEach((e) => {
        e.track.stop();
      }),
      this.peer.close(),
      (this.resource = null),
      (this.peer = null);
  }
  getResourceUrl() {
    return this.resource
      ? Promise.resolve(this.resource)
      : new Promise((e) => {
          this.resourceResolve = e;
        });
  }
  async getResourceExtensions() {
    return await this.getResourceUrl(), this.extensions;
  }
}
export { o as WHIPClient };
