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

        pc.current.addTransceiver("video", {
            direction: "recvonly", // or "sendrecv" if you also plan to send video
        });

        const transceivers = pc.current.getTransceivers();
        for (const transceiver of transceivers) {
            if (transceiver.receiver.track.kind === "video") {
                // 1. Get all video capabilities
                const cap = RTCRtpSender.getCapabilities("video");
                if (!cap) continue;

                // 2. Find H.264 codecs
                // mimeType can be "video/H264" or sometimes "video/h264"
                const h264Codecs = cap.codecs.filter((c) =>
                    c.mimeType.toLowerCase() === "video/H264"
                );
                if (h264Codecs.length) {
                    // 3. Reorder so that H.264 is at the front, followed by the rest
                    const preferred = [
                        ...h264Codecs,
                        ...cap.codecs.filter((c) => c.mimeType.toLowerCase() !== "video/H264"),
                    ];
                    // 4. Apply the codec preferences to this transceiver
                    transceiver.setCodecPreferences(preferred);
                    console.log("Preferred H.264 for this transceiver");
                }
            }
        }


        const offer = await pc.current.createOffer({
            iceRestart: true,
            offerToReceiveAudio: false,
            offerToReceiveVideo: true
        });

        await pc.current.setLocalDescription(offer);

        const message = JSON.stringify({
            type: "offer",
            sdp: offer.sdp
        });

        ws.current.send(message);
        console.log("Offer sent:", message);
    };

    return (
        <div>
            <button onClick={sendOffer}>Send Offer</button>;
            <video ref={videoRef} autoPlay playsInline style={{ width: "100%", maxHeight: "800px", background: "black" }} />
        </div>
    )

};

export default WebRTCClient;

