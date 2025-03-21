import React, { useState, useEffect, useRef } from "react";

const WebRTCClient = () => {
    const ws = useRef(null);
    const pc = useRef(null);
    const videoRef = useRef(null);

    useEffect(() => {
        const caps = RTCRtpSender.getCapabilities("video");
        console.debug("WebRTC: Browser video capabilities:", caps);

        ws.current = new WebSocket("ws://192.168.1.41:10001/rtc");

        ws.current.onopen = () => {
            console.log("WebRTC: Connected to signalling websocket server")
        };

        ws.current.onmessage = (event) => {
            const data = JSON.parse(event.data)
            console.log('WebRTC: Received from signalling server:', data)

            if (data.type === "answer" && pc.current) {
                console.log('WebRTC: Received answer from signalling server')
                pc.current.setRemoteDescription(new RTCSessionDescription(data))
            } else if (data.type === "candidate" && pc.current) {
                console.log('WebRTC: Received ICE candidate from signalling server')
                const candidate = new RTCIceCandidate({
                    candidate: data.candidate,
                    sdpMid: data.sdpMid,
                    sdpMLineIndex: data.sdpMLineIndex
                });
                pc.current.addIceCandidate(candidate).catch(console.error);
            }
        };

        return () => {
            if (ws.current) {
                ws.current.close();
            }
            if (pc.current) {
                pc.current.close();
            }
        };
    }, []);

    const sendOffer = async () => {
        if (!ws.current || ws.current.readyState !== WebSocket.OPEN) {
            console.error("WebRTC: Provider WebRTC signalling server webSocket is not connected!")
            return;
        }

        pc.current = new RTCPeerConnection({
            iceServers: [{ urls: "stun:stun.l.google.com:19302" }]
        });

        pc.current.ontrack = (event) => {
            console.log('WebRTC: Received remote track: ', event)
            if (event.streams.length > 0) {
                console.log('WebRTC: There are track streams available!')
                videoRef.current.srcObject = event.streams[0]
                console.log("WebRTC: ✅ Remote video stream set")
                // event.track.enabled = true
                console.log('WebRTC: Attempting to force video playback')
                videoRef.current.play().catch(e => console.error("🔴 Failed to play video:", e))

            } else {
                console.warn("WebRTC: No video track in event");
            }
        };

        pc.current.onicecandidate = (event) => {
            if (event.candidate) {
                const message = JSON.stringify({
                    type: "candidate",
                    candidate: event.candidate
                });
                ws.current.send(message);
                console.log("WebRTC: Sent ICE candidate to signalling server: ", message)
            }
        };

        pc.current.oniceconnectionstatechange = () => {
            console.log("WebRTC: ICE connection state: ", pc.current.iceConnectionState);
        };

        const transceiver = pc.current.addTransceiver("video", {
            direction: "recvonly"
        })

        if (isChrome()) {
            if (transceiver.setCodecPreferences) {
                console.log('WebRTC: Browser supports setting WebRTC codec preferences, trying to force H.264.')
                const capabilities = RTCRtpReceiver.getCapabilities("video");
                const h264Codecs = capabilities.codecs.filter(codec =>
                    codec.mimeType.toLowerCase() === "video/vp8"
                )
                console.log("CODECS")
                console.log(h264Codecs)
                // Force the transceiver to prefer H.264 if available
                if (h264Codecs.length) {
                    transceiver.setCodecPreferences(h264Codecs)
                } else {
                    console.warn("WebRTC: H.264 not supported in this browser's codecs.")
                }
            }
        }

        const offer = await pc.current.createOffer({
            iceRestart: true,
            offerToReceiveAudio: false,
            offerToReceiveVideo: true
        })

        if (isFirefox() || isSafari()) {
            console.log('WebRTC: Trying to prefer H.264 codec for Firefox by re-writing offer SDP')
            offer.sdp = preferCodec(offer.sdp, "H264")
        }

        await pc.current.setLocalDescription(offer)

        const message = JSON.stringify({
            type: "offer",
            sdp: offer.sdp
        });

        ws.current.send(message)
        console.log("Offer sent:", message)
    };

    const preferCodec = (sdp, codec = "VP9") => {
        const lines = sdp.split("\r\n")
        let mLineIndex = -1
        let codecPayloadType = null

        console.log("LINES")
        console.log(lines)
        for (let i = 0; i < lines.length; i++) {
            if (lines[i].startsWith("m=video")) {
                mLineIndex = i
            }
            if (lines[i].toLowerCase().includes(`a=rtpmap`) && lines[i].includes(codec)) {
                codecPayloadType = lines[i].match(/:(\d+) /)[1]
                break
            }
        }

        if (mLineIndex === -1 || codecPayloadType === null) {
            console.warn(`WebRTC: ${codec} codec not found in SDP`)
            return sdp;
        }

        console.log("CHANGING TO PAYLOAD TYPE " + codecPayloadType)

        // const mLineParts = lines[mLineIndex].split(" ");
        const newMLine = [lines[mLineIndex].split(" ")[0], lines[mLineIndex].split(" ")[1], lines[mLineIndex].split(" ")[2], codecPayloadType]
            .concat(lines[mLineIndex].split(" ").slice(3).filter(pt => pt !== codecPayloadType))

        lines[mLineIndex] = newMLine.join(" ")
        return lines.join("\r\n")
    };

    function agentHas(keyword) {
        return navigator.userAgent.toLowerCase().search(keyword.toLowerCase()) > -1;
    }

    function isSafari() {
        return (!!window.ApplePaySetupFeature || !!window.safari) && agentHas("Safari") && !agentHas("Chrome") && !agentHas("CriOS");
    }

    function isChrome() {
        return agentHas("CriOS") || agentHas("Chrome") || !!window.chrome;
    }

    function isFirefox() {
        return agentHas("Firefox") || agentHas("FxiOS") || agentHas("Focus");
    }

    return (
        <div>
            <button onClick={sendOffer}>Send Offer</button>;
            <video ref={videoRef} autoPlay playsInline style={{ width: "360px", height: "640px", background: "black" }} />
        </div>
    )

};

export default WebRTCClient;

