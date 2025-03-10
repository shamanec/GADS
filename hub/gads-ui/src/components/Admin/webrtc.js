import React, { useState, useEffect, useRef } from "react";

const WebRTCClient = () => {
    const ws = useRef(null);
    const pc = useRef(null);
    const videoRef = useRef(null);

    useEffect(() => {
        const caps = RTCRtpSender.getCapabilities("video");
        console.log("Browser video capabilities:", caps);

        ws.current = new WebSocket("ws://192.168.1.41:10001/device/00008030-0018386C1106402E/webrtc");

        ws.current.onopen = () => {
            console.log("Connected to WebSocket server");
        };

        ws.current.onmessage = (event) => {
            const data = JSON.parse(event.data);
            console.log("Received from server:", data);

            if (data.type === "answer" && pc.current) {
                console.log("DATA TYPE IS ANSWER")
                pc.current.setRemoteDescription(new RTCSessionDescription(data));
            } else if (data.type === "candidate" && pc.current) {
                console.log("DATA TYPE IS CANDIDATE, ADDING")
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
            console.error("WebSocket is not connected");
            return;
        }

        pc.current = new RTCPeerConnection({
            iceServers: [{ urls: "stun:stun.l.google.com:19302" }]
        });

        pc.current.ontrack = (event) => {
            console.log("Received remote track:", event);
            if (videoRef.current && event.streams.length > 0) {
                videoRef.current.srcObject = event.streams[0];
                console.log("✅ Remote video stream set");

                const track = event.track;
                console.log(`🎬 Track state: enabled=${track.enabled}, muted=${track.muted}`);
                event.track.enabled = true
                videoRef.current.play().catch(e => console.error("🔴 Failed to play video:", e));

            } else {
                console.warn("⚠️ No video track in event");
            }
        };

        pc.current.onicecandidate = (event) => {
            if (event.candidate) {
                const message = JSON.stringify({
                    type: "candidate",
                    candidate: event.candidate
                });
                ws.current.send(message);
                console.log("Sent ICE candidate:", message);
            }
        };

        pc.current.oniceconnectionstatechange = () => {
            console.log("ICE connection state:", pc.current.iceConnectionState);
        };

        const transceiver = pc.current.addTransceiver("video", {
            direction: "recvonly", // or "sendrecv" if you also plan to send video
        });

        if (transceiver.setCodecPreferences) {
            const capabilities = RTCRtpReceiver.getCapabilities("video");
            const h264Codecs = capabilities.codecs.filter(codec =>
                codec.mimeType.toLowerCase() === "video/h264"
            )

            // Force the transceiver to prefer H.264 if available
            if (h264Codecs.length) {
                transceiver.setCodecPreferences(h264Codecs);
            } else {
                console.warn("H.264 not supported in this browser's codecs.");
            }
        } else {
            console.warn("No setCodecPreferences() support; falling back to SDP rewrite.");
        }

        const offer = await pc.current.createOffer({
            iceRestart: true,
            offerToReceiveAudio: false,
            offerToReceiveVideo: true
        });

        // if (!transceiver.setCodecPreferences) {
        //     console.log("PREFERRING CODEC")
        //     offer.sdp = preferCodec(offer.sdp, "VP9");
        // }

        await pc.current.setLocalDescription(offer);

        const message = JSON.stringify({
            type: "offer",
            sdp: offer.sdp
        });

        ws.current.send(message);
        console.log("Offer sent:", message);
    };

    const preferCodec = (sdp, codec = "VP9") => {
        const lines = sdp.split("\r\n");
        let mLineIndex = -1;
        let codecPayloadType = null;

        for (let i = 0; i < lines.length; i++) {
            if (lines[i].startsWith("m=video")) {
                mLineIndex = i;
            }
            if (lines[i].toLowerCase().includes(`a=rtpmap`) && lines[i].includes(codec)) {
                codecPayloadType = lines[i].match(/:(\d+) /)[1];
                break;
            }
        }

        if (mLineIndex === -1 || codecPayloadType === null) {
            console.warn(`${codec} codec not found in SDP`);
            return sdp;
        }

        console.log("CODEC TYPE")
        console.log(codecPayloadType)

        const mLineParts = lines[mLineIndex].split(" ");
        const newMLine = [lines[mLineIndex].split(" ")[0], lines[mLineIndex].split(" ")[1], lines[mLineIndex].split(" ")[2], codecPayloadType]
            .concat(lines[mLineIndex].split(" ").slice(3).filter(pt => pt !== codecPayloadType));

        lines[mLineIndex] = newMLine.join(" ");
        return lines.join("\r\n");
    };

    return (
        <div>
            <button onClick={sendOffer}>Send Offer</button>;
            <video ref={videoRef} autoPlay playsInline style={{ width: "540px", maxHeight: "1120px", background: "black" }} />
        </div>
    )

};

export default WebRTCClient;

