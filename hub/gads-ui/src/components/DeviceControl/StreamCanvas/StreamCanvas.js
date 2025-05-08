import { useEffect, useRef, useState } from 'react'
import './StreamCanvas.css'
import { Button, Divider, Grid, Tooltip } from '@mui/material'
import HomeIcon from '@mui/icons-material/Home'
import LockOpenIcon from '@mui/icons-material/LockOpen'
import LockIcon from '@mui/icons-material/Lock'
import KeyboardArrowLeftIcon from '@mui/icons-material/KeyboardArrowLeft'
import KeyboardArrowRightIcon from '@mui/icons-material/KeyboardArrowRight'
import KeyboardArrowUpIcon from '@mui/icons-material/KeyboardArrowUp'
import KeyboardArrowDownIcon from '@mui/icons-material/KeyboardArrowDown'
import { api } from '../../../services/api.js'
import StreamSettings from './StreamSettings.js'
import { useSnackbar } from '../../../contexts/SnackBarContext.js'

export default function StreamCanvas({ deviceData, shouldShowStream }) {
    // WebRTC refs
    const ws = useRef(null)
    const pc = useRef(null)
    const videoRef = useRef(null)
    const videoDimensionsRef = useRef()
    const [remoteStream, setRemoteStream] = useState(null)

    const { showSnackbar } = useSnackbar()
    const [isPortrait, setIsPortrait] = useState(true)
    const [canvasDimensions, setCanvasDimensions] = useState({
        width: 0,
        height: 0
    })

    let deviceX = parseInt(deviceData.screen_width, 10)
    let deviceY = parseInt(deviceData.screen_height, 10)
    let deviceScreenRatio = deviceX / deviceY
    let deviceLandscapeScreenRatio = deviceY / deviceX
    let deviceOS = deviceData.os
    let udid = deviceData.udid
    let useWebRTCVideo = deviceData.use_webrtc_video
    let webRTCVideoCodec = deviceData.webrtc_video_codec

    let streamUrl = ''
    if (deviceData.os === 'ios') {
        // streamUrl = `http://192.168.1.41:10000/device/${deviceData.udid}/ios-stream-mjpeg`
        streamUrl = `/device/${deviceData.udid}/ios-stream-mjpeg`
    } else {
        // streamUrl = `http://192.168.1.41:10000/device/${deviceData.udid}/android-stream-mjpeg`
        streamUrl = `/device/${deviceData.udid}/android-stream-mjpeg`
    }

    useEffect(() => {
        function handleKeyDown(event) {
            // Check if there is an active element to avoid capturing keyboard events
            // When we want to be typing somewhere else, not send to the device
            const activeElement = document.activeElement
            const isInputFocused = activeElement && (
                activeElement.tagName === 'INPUT' ||
                activeElement.tagName === 'TEXTAREA' ||
                activeElement.isContentEditable
            )

            // Don't process keyboard events when user is typing somewhere
            if (isInputFocused) {
                console.log(`Ignoring keyboard input because we have a focused element with a tagName '${activeElement.tagName}' and editable state '${activeElement.isContentEditable}'`)
                return
            }

            // Handle normal chars, special chars, Enter and Backspace, ignore stuff like
            //  Shift, Ctrl, Alt, F1-F12, etc.
            const key = event.key

            // Prevent typing the 'v' char when doing Ctrl/Cmd + V
            if ((event.ctrlKey || event.metaKey) && key.toLowerCase() === 'v') {
                return
            }

            if (key.length === 1) {
                sendKeyPress(key)
            } else if (key === 'Enter') {
                sendKeyPress('\n')
            } else if (key === 'Backspace') {
                sendKeyPress('\b')
            } else {
                return
            }
        }

        async function handlePaste(event) {
            const activeElement = document.activeElement;
            const isInputFocused = activeElement && (
                activeElement.tagName === 'INPUT' ||
                activeElement.tagName === 'TEXTAREA' ||
                activeElement.isContentEditable
            );

            if (isInputFocused) {
                return;
            }

            // Get clipboard data from the paste event
            const pastedText = event.clipboardData?.getData('text')
            if (pastedText) {
                event.preventDefault()
                sendKeyPress(pastedText) // Send the paste event text similar to typing in handleKeyDown
            }
        }

        // Register the event listener and remove it on component unmounted
        window.addEventListener('keydown', handleKeyDown)
        window.addEventListener('paste', handlePaste)
        return () => {
            window.removeEventListener('keydown', handleKeyDown)
            window.removeEventListener('paste', handlePaste)
        }
    }, [])

    function sendKeyPress(key) {
        const deviceURL = `/device/${udid}/typeText`
        const jsonData = JSON.stringify({ text: key })

        api.post(deviceURL, jsonData)
            .catch((error) => {
                console.error('Key press failed', error)
                showCustomSnackbarError('Key press failed!')
            })
    }

    const handleOrientationButtonClick = (isPortrait) => {
        setIsPortrait(isPortrait)
        updateCanvasDimensions(isPortrait)
    }

    // Handles orientation/resizing only
    useEffect(() => {
        const handleResize = () => {
            // Only update if the ref still exists
            if (videoDimensionsRef.current) {
                updateCanvasDimensions(isPortrait)
            }
        }

        handleResize()

        window.addEventListener('resize', handleResize)
        return () => window.removeEventListener('resize', handleResize)
    }, [])

    // Handles starting/stopping the WebRTC connection
    useEffect(() => {
        if (!useWebRTCVideo) {
            const imgElement = document.getElementById('image-stream')
            // If MJPEG, just set the <img> src once
            if (shouldShowStream) {
                imgElement.src = streamUrl
            } else {
                imgElement.src = ''
            }
            return;
        }

        if (shouldShowStream) {
            setupWebRTCVideo()
        }

        return () => {
            // Only tear down if user hides the stream or unmounts
            if (ws.current) {
                ws.current.close()
                ws.current = null
            }
            if (pc.current) {
                pc.current.close()
                pc.current = null
            }
        };
    }, [useWebRTCVideo, shouldShowStream])

    useEffect(() => {
        if (useWebRTCVideo) {
            videoRef.current.srcObject = remoteStream
        }
    }, [remoteStream, videoRef, isPortrait])

    const updateCanvasDimensions = (isPortrait) => {
        let calculatedWidth, calculatedHeight
        if (isPortrait) {
            calculatedHeight = window.innerHeight * 0.7
            calculatedWidth = calculatedHeight * deviceScreenRatio
        } else {
            calculatedWidth = window.innerWidth * 0.4
            calculatedHeight = calculatedWidth / deviceLandscapeScreenRatio
        }


        videoDimensionsRef.current.style.width = calculatedWidth + 'px'
        videoDimensionsRef.current.style.height = calculatedHeight + 'px'

        setCanvasDimensions({
            width: calculatedWidth,
            height: calculatedHeight
        })
    }

    function setupWebRTCVideo() {
        const caps = RTCRtpSender.getCapabilities('video')
        console.debug(`WebRTC: Browser video capabilities: ${caps}`)

        const protocol = window.location.protocol
        let wsType = 'ws'
        if (protocol === 'https:') {
            wsType = 'wss'
        }
        let socketUrl = `${wsType}://${window.location.host}/devices/control/${udid}/webrtc`
        //let socketUrl = `${wsType}://192.168.1.41:10000/devices/control/${udid}/webrtc`
        ws.current = new WebSocket(socketUrl)

        ws.current.onopen = () => {
            console.log('WebRTC: Connected to signalling websocket server')
            sendOffer()
        };

        ws.current.onmessage = (event) => {
            const data = JSON.parse(event.data)
            console.log(`WebRTC: Received from signalling server: ${data}`)

            if (data.type === 'answer' && pc.current) {
                console.log('WebRTC: Received answer from signalling server')
                pc.current.setRemoteDescription(new RTCSessionDescription(data))
            } else if (data.type === "candidate" && pc.current) {
                console.log('WebRTC: Received ICE candidate from signalling server')
                const candidate = new RTCIceCandidate({
                    candidate: data.candidate,
                    sdpMid: data.sdpMid,
                    sdpMLineIndex: data.sdpMLineIndex
                });
                pc.current.addIceCandidate(candidate).catch(console.error)
            }
        }

        ws.current.onerror = (error) => {
            console.error("WebSocket error:", error)
        }
    }

    const sendOffer = async () => {
        console.log('Sending offer')
        if (!ws.current || ws.current.readyState !== WebSocket.OPEN) {
            console.log(ws.current)
            console.log(ws.current.readyState)
            console.error('WebRTC: Provider WebRTC signalling server webSocket is not connected!')
            return;
        }

        pc.current = new RTCPeerConnection({
            iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
        })
        // pc.current = new RTCPeerConnection()

        pc.current.ontrack = (event) => {
            console.log(`WebRTC: Received remote track: ${event}`)
            if (videoRef.current && event.streams.length > 0) {
                console.log('WebRTC: There are track streams available!')
                videoRef.current.srcObject = event.streams[0]
                setRemoteStream(event.streams[0])
                console.log('WebRTC: ✅ Remote video stream set')
            } else {
                console.warn('WebRTC: No video track in event')
            }
        };

        pc.current.onicecandidate = (event) => {
            if (event.candidate) {
                const message = JSON.stringify({
                    type: 'candidate',
                    candidate: event.candidate
                });
                ws.current.send(message);
                console.log(`WebRTC: Sent ICE candidate to signalling server: ${message}`)
            }
        };

        pc.current.oniceconnectionstatechange = () => {
            console.log(`WebRTC: ICE connection state: ${pc.current.iceConnectionState}`)
        };

        const transceiver = pc.current.addTransceiver('video', {
            direction: "recvonly"
        })

        if (isChrome()) {
            if (transceiver.setCodecPreferences) {
                console.log('WebRTC: Browser supports setting WebRTC codec preferences, trying to force H.264.')
                const capabilities = RTCRtpReceiver.getCapabilities("video");
                const foundCodecs = capabilities.codecs.filter(codec =>
                    codec.mimeType.toLowerCase() === `video/${webRTCVideoCodec}`
                )
                console.log(`WebRTC: Found codecs with preference for ${webRTCVideoCodec}`)
                console.log(foundCodecs)
                if (foundCodecs.length) {
                    transceiver.setCodecPreferences(foundCodecs)
                } else {
                    console.warn(`WebRTC: '${webRTCVideoCodec}' not supported in this browser's codecs.`)
                }
            }
        }

        const offer = await pc.current.createOffer({
            iceRestart: true,
            offerToReceiveAudio: false,
            offerToReceiveVideo: true
        })

        if (isFirefox() || isSafari()) {
            console.log(`WebRTC: Trying to prefer '${webRTCVideoCodec}' codec for Firefox/Safari by re-writing offer SDP`)
            offer.sdp = preferCodec(offer.sdp, `${webRTCVideoCodec}`.toUpperCase)
        }

        await pc.current.setLocalDescription(offer)

        const message = JSON.stringify({
            type: 'offer',
            sdp: offer.sdp
        })

        ws.current.send(message)
        console.log(`Offer sent: ${message}`)
    };

    const preferCodec = (sdp, codec = 'VP9') => {
        const lines = sdp.split('\r\n')
        let mLineIndex = -1
        let codecPayloadType = null

        for (let i = 0; i < lines.length; i++) {
            if (lines[i].startsWith('m=video')) {
                mLineIndex = i
            }
            if (lines[i].toLowerCase().includes(`a=rtpmap`) && lines[i].includes(codec)) {
                codecPayloadType = lines[i].match(/:(\d+) /)[1]
                break
            }
        }

        if (mLineIndex === -1 || codecPayloadType === null) {
            console.warn(`WebRTC: ${codec} codec not found in SDP`)
            return sdp
        }

        // const mLineParts = lines[mLineIndex].split(" ");
        const newMLine = [lines[mLineIndex].split(' ')[0], lines[mLineIndex].split(' ')[1], lines[mLineIndex].split(' ')[2], codecPayloadType]
            .concat(lines[mLineIndex].split(" ").slice(3).filter(pt => pt !== codecPayloadType))

        lines[mLineIndex] = newMLine.join(' ')
        return lines.join("\r\n")
    };

    // Check for specific keyword in browser agent
    function agentHas(keyword) {
        return navigator.userAgent.toLowerCase().search(keyword.toLowerCase()) > -1
    }

    // Check if current browser is Safari
    function isSafari() {
        return (!!window.ApplePaySetupFeature || !!window.safari) && agentHas('Safari') && !agentHas('Chrome') && !agentHas('CriOS')
    }

    // Check if current browser is Chrome
    function isChrome() {
        return agentHas('CriOS') || agentHas('Chrome') || !!window.chrome
    }

    // Check if current browser is Firefox
    function isFirefox() {
        return agentHas('Firefox') || agentHas('FxiOS') || agentHas('Focus')
    }

    const showCustomSnackbarError = (message) => {
        showSnackbar({
            message: message,
            severity: 'error',
            duration: 3000,
        })
    }

    return (
        <Grid
            spacing={1}
            direction='row'
            display='flex'
            justifyContent='center'
            alignItems='flex-start'
        >

            <div
                id='phone-imitation'
            >
                <h3
                    style={{
                        color: '#2f3b26',
                        display: 'flex',
                        fontFamily: 'Verdana',
                        justifyContent: 'center'
                    }}
                >{deviceData.model}</h3>
                <div
                    ref={videoDimensionsRef}
                    id='stream-div'
                    style={{
                        position: 'relative',
                    }}
                >
                    <Canvas></Canvas>
                    {useWebRTCVideo ? <VideoStream></VideoStream> : <Stream></Stream>}
                </div>
                <Grid
                    height='30px'
                    display='flex'
                    justifyContent='center'
                >
                </Grid>
            </div >

            <Grid
                direction='column'
                width='150px'
                marginLeft='10px'
                spacing={1}
                container
            >
                <Grid item>
                    <Tooltip
                        title='This does not change the orientation of the device itself, just updates the UI if the device orientation is already changed'
                        arrow
                        placement='top'
                    >
                        <Button
                            variant={'contained'}
                            color={'secondary'}
                            onClick={() => handleOrientationButtonClick(true)}
                            disabled={isPortrait}
                            sx={{ width: '100%' }}
                        >
                            Portrait
                        </Button>
                    </Tooltip>
                </Grid>
                <Grid item>
                    <Tooltip
                        title='This does not change the orientation of the device itself, just updates the UI if the device orientation is already changed'
                        arrow
                        placement='top'
                    >
                        <Button
                            variant={'contained'}
                            color={'secondary'}
                            onClick={() => handleOrientationButtonClick(false)}
                            disabled={!isPortrait}
                            sx={{ width: '100%' }}
                        >
                            Landscape
                        </Button>
                    </Tooltip>
                </Grid>
                <Grid item>
                    <Divider></Divider>
                </Grid>
                <Grid item>
                    <Button
                        onClick={() => homeButton()}
                        className='canvas-buttons'
                        startIcon={<HomeIcon />}
                        variant='contained'
                        style={{
                            fontWeight: 'bold',
                            color: '#9ba984',
                            backgroundColor: '#2f3b26',
                            width: '100%'
                        }}
                    >Home</Button>
                </Grid>
                <Grid item>
                    <Button
                        onClick={() => lockButton()}
                        className='canvas-buttons'
                        startIcon={<LockIcon />}
                        variant='contained'
                        style={{
                            fontWeight: 'bold',
                            color: '#9ba984',
                            backgroundColor: '#2f3b26',
                            width: '100%'
                        }}
                    >Lock</Button>
                </Grid>
                <Grid item>
                    <Button
                        onClick={() => unlockButton()}
                        className='canvas-buttons'
                        startIcon={<LockOpenIcon />}
                        variant='contained'
                        style={{
                            fontWeight: 'bold',
                            color: '#9ba984',
                            backgroundColor: '#2f3b26',
                            width: '100%'
                        }}
                    >Unlock</Button>
                </Grid>
                <Grid item>
                    <Button
                        onClick={() => swipeLeft()}
                        className='canvas-buttons'
                        variant='contained'
                        startIcon={<KeyboardArrowRightIcon />}
                        style={{
                            fontWeight: 'bold',
                            color: '#9ba984',
                            backgroundColor: '#2f3b26',
                            width: '100%'
                        }}
                    >Swipe</Button>
                </Grid>
                <Grid item>
                    <Button
                        onClick={() => swipeRight()}
                        className='canvas-buttons'
                        variant='contained'
                        startIcon={<KeyboardArrowLeftIcon />}
                        style={{
                            fontWeight: 'bold',
                            color: '#9ba984',
                            backgroundColor: '#2f3b26',
                            width: '100%'
                        }}
                    >Swipe</Button>
                </Grid>
                <Grid item>
                    <Button
                        onClick={() => swipeUp()}
                        className='canvas-buttons'
                        variant='contained'
                        startIcon={<KeyboardArrowDownIcon />}
                        style={{
                            fontWeight: 'bold',
                            color: '#9ba984',
                            backgroundColor: '#2f3b26',
                            width: '100%'
                        }}
                    >Swipe</Button>
                </Grid>
                <Grid item>
                    <Button
                        onClick={() => swipeDown()}
                        className='canvas-buttons'
                        variant='contained'
                        startIcon={<KeyboardArrowUpIcon />}
                        style={{
                            fontWeight: 'bold',
                            color: '#9ba984',
                            backgroundColor: '#2f3b26',
                            width: '100%'
                        }}
                    >Swipe</Button>
                </Grid>
                <Grid item>
                    {!useWebRTCVideo && <StreamSettings deviceData={deviceData} />}
                </Grid>
            </Grid>
        </Grid >
    )

    function Canvas() {
        var tapStartAt = 0
        var coord1
        var coord2

        function getCursorCoordinates(event) {
            const rect = event.currentTarget.getBoundingClientRect()
            const x = event.clientX - rect.left
            const y = event.clientY - rect.top
            return [x, y]
        }

        function handleMouseDown(event) {
            tapStartAt = (new Date()).getTime()
            coord1 = getCursorCoordinates(event)
        }

        function handleMouseUp(event) {
            coord2 = getCursorCoordinates(event)
            // get the time of finishing the click on the canvas
            var tapEndAt = (new Date()).getTime()

            console.log('Tap on ' + coord1[0] + 'x' + coord1[1] + ', released on ' + coord2[0] + 'x' + coord2[1])

            var mouseEventsTimeDiff = tapEndAt - tapStartAt

            // if the difference of time between click down and click up is more than 500ms assume it is a swipe, not a tap
            // to allow flick swipes we also check the difference between the gesture coordinates
            // x1, y1 = mousedown coordinates
            // x2, y2 = mouseup coordinates
            // if x2 > x1*1.1 - it is probably a swipe left to right
            // if x2 < x1*0.9 - it is probably a swipe right to left
            // if y2 < y1*0.9 - it is probably a swipe bottom to top
            // if y2 > y1*1.1 - it is probably a swipe top to bottom
            if (mouseEventsTimeDiff > 500 && coord2[0] > coord1[0] * 1.1 || coord2[0] < coord1[0] * 0.9 || coord2[1] < coord1[1] * 0.9 || coord2[1] > coord1[1] * 1.1) {
                swipeCoordinates(coord1, coord2)
            } else if (mouseEventsTimeDiff < 500) {
                tapCoordinates(coord1)
            } else {
                touchAndHoldCoordinates(coord1)
            }
        }

        return (
            <canvas
                id='actions-canvas'
                onMouseDown={handleMouseDown}
                onMouseUp={handleMouseUp}
                style={{ position: 'absolute', zIndex: 2, width: '100%', height: '100%' }}
            ></canvas>
        )
    }

    function Stream() {
        return (
            <img
                id='image-stream'
                width="100%"
                height="100%"
                style={{ display: 'block' }}
                src={shouldShowStream ? streamUrl : ''}
            ></img>
        )
    }

    function VideoStream() {
        return (
            <video ref={videoRef} autoPlay playsInline style={{ background: "black", display: 'block', zIndex: 1, width: '100%', height: '100%' }} />
        )
    }

    function swipeUp() {
        let startX = canvasDimensions.width / 2
        let endX = canvasDimensions.width / 2
        let startY = canvasDimensions.height - (canvasDimensions.height * 0.75)
        let endY = canvasDimensions.height - (canvasDimensions.height * 0.25)
        let coord1 = [startX, startY]
        let coord2 = [endX, endY]
        swipeCoordinates(coord1, coord2)
    }

    function swipeDown() {
        let startX = canvasDimensions.width / 2
        let endX = canvasDimensions.width / 2
        let startY = canvasDimensions.height - (canvasDimensions.height * 0.25)
        let endY = canvasDimensions.height - (canvasDimensions.height * 0.75)
        let coord1 = [startX, startY]
        let coord2 = [endX, endY]
        swipeCoordinates(coord1, coord2)
    }

    function swipeLeft() {
        let startX = canvasDimensions.width - (canvasDimensions.width * 0.20)
        let endX = canvasDimensions.width - (canvasDimensions.width * 0.80)
        let startY = canvasDimensions.height / 2
        let endY = canvasDimensions.height / 2
        let coord1 = [startX, startY]
        let coord2 = [endX, endY]
        swipeCoordinates(coord1, coord2)
    }

    function swipeRight() {
        let startX = canvasDimensions.width - (canvasDimensions.width * 0.80)
        let endX = canvasDimensions.width - (canvasDimensions.width * 0.20)
        let startY = canvasDimensions.height / 2
        let endY = canvasDimensions.height / 2
        let coord1 = [startX, startY]
        let coord2 = [endX, endY]
        swipeCoordinates(coord1, coord2)
    }

    // tap using coordinates
    function tapCoordinates(pos) {
        // set initial x and y tap coordinates
        let x = pos[0]
        let y = pos[1]

        let finalX = (x / canvasDimensions.width) * deviceX
        let finalY = (y / canvasDimensions.height) * deviceY
        // If its portrait we keep the x and y as is
        if (!isPortrait) {
            // If its landscape
            // And its Android we still keep the x and y as is because for Android Appium does the coordinates are actually corresponding to the view
            // If we are in portrait X is X and Y is Y and when we are in landscape its the same
            if (deviceOS === 'android') {
                finalX = (x / canvasDimensions.height) * deviceX
                finalY = (y / canvasDimensions.width) * deviceY
            }
            if (deviceOS === 'ios') {
                finalX = deviceX - ((y / canvasDimensions.height) * deviceX)
                finalY = (x / canvasDimensions.width) * deviceY
            }
        }

        let jsonData = JSON.stringify({
            'x': finalX,
            'y': finalY
        })

        let deviceURL = `/device/${udid}`

        api.post(deviceURL + '/tap', jsonData)
            .then(response => {
                if (response.status === 404) {
                    return
                }
            })
            .catch((error) => {
                if (error.response) {
                    if (error.response.status === 404) {
                        showCustomSnackbarError('Tap failed - Appium session has expired!')
                    } else {
                        showCustomSnackbarError('Tap failed!')
                    }
                } else {
                    showCustomSnackbarError('Tap failed!')
                }
            })
    }

    function touchAndHoldCoordinates(pos) {
        // set initial x and y tap coordinates
        let x = pos[0]
        let y = pos[1]

        let finalX = (x / canvasDimensions.width) * deviceX
        let finalY = (y / canvasDimensions.height) * deviceY

        // If its portrait we keep the x and y as is
        if (!isPortrait) {
            // If its landscape
            // And its Android we still keep the x and y as is because for Android Appium does the coordinates are actually corresponding to the view
            // If we are in portrait X is X and Y is Y and when we are in landscape its the same
            if (deviceOS === 'android') {
                finalX = (x / canvasDimensions.height) * deviceX
                finalY = (y / canvasDimensions.width) * deviceY
            }
            if (deviceOS === 'ios') {
                finalX = deviceX - ((y / canvasDimensions.height) * deviceX)
                finalY = (x / canvasDimensions.width) * deviceY
            }
        }

        let jsonData = JSON.stringify({
            'x': finalX,
            'y': finalY
        })

        let deviceURL = `/device/${udid}`

        api.post(deviceURL + '/touchAndHold', jsonData)
            .then(() => { })
            .catch((error) => {
                if (error.response) {
                    if (error.response.status === 404) {
                        showCustomSnackbarError('Touch & hold failed - Appium session has expired!')
                    } else {
                        showCustomSnackbarError('Touch & hold failed!')
                    }
                } else {
                    showCustomSnackbarError('Touch & hold failed!')
                }
            })
    }

    function swipeCoordinates(coord1, coord2) {
        var firstCoordX = coord1[0]
        var firstCoordY = coord1[1]
        var secondCoordX = coord2[0]
        var secondCoordY = coord2[1]

        // Set up the portrait tap coordinates
        // We divide the current coordinate by the canvas size to get the ratio
        // Then we multiply by the actual device width or height to get the actual coordinates on the device that we should send
        let firstXFinal = (firstCoordX / canvasDimensions.width) * deviceX
        let firstYFinal = (firstCoordY / canvasDimensions.height) * deviceY
        let secondXFinal = (secondCoordX / canvasDimensions.width) * deviceX
        let secondYFinal = (secondCoordY / canvasDimensions.height) * deviceY

        // If in landscape we need to do different recalculations
        if (!isPortrait) {
            // If the device is android we just reverse the calculations
            // For X we divide the coordinate by the canvas height to get the correct ratio
            // And for Y we divide the coordinate by the canvas width to get the correct ratio
            // Multiplication is the same as for portrait because Appium for Android follows some actual logic
            if (deviceOS === 'android') {
                firstXFinal = (firstCoordX / canvasDimensions.height) * deviceX
                firstYFinal = (firstCoordY / canvasDimensions.width) * deviceY
                secondXFinal = (secondCoordX / canvasDimensions.height) * deviceX
                secondYFinal = (secondCoordY / canvasDimensions.width) * deviceY
            } else {
                // For iOS its complete sh*t
                // NB: All calculations below are when the device is on its right side landscape(your left)
                // For custom WDA the 0 X coordinate is at the bottom of the canvas
                // And the 0 Y coordinate is at the left end of the canvas
                // This means that they kinda follow the portrait logic but inverted based on the side it is in landscape
                // Imagine a device that is X:Y=100:200
                // If you swipe vertically from bottom to the middle you are essentially swiping coordinates on the device X:0 and X: 50
                // And if you swipe horizontally from left to middle you are essentially swiping coordinates on the device Y: 0 and Y: 100
                // But on the canvas those are Y coordinates
                // And this is where it gets funky
                // For X we get the ratio from the canvas Y coordinate and the canvas height
                // And we multiply it by the device width because that is what it corresponds to
                // Then we subtract the value from the actual deviceX because the device width starts from the bottom of the canvas height
                firstXFinal = deviceX - ((firstCoordY / canvasDimensions.height) * deviceX)
                // For Y we get the ratio from the canvas X coordinate and the canvas width
                // And we multiply it by the device height because that is what it corresponds to
                firstYFinal = (firstCoordX / canvasDimensions.width) * deviceY
                // Same goes for the other two coordinates when the mouse is released
                secondXFinal = deviceX - ((secondCoordY / canvasDimensions.height) * deviceX)
                secondYFinal = (secondCoordX / canvasDimensions.width) * deviceY
            }
        }
        console.log('Swipe is ' + firstXFinal + ':' + firstYFinal + '-' + secondXFinal + ':' + secondYFinal)

        let jsonData = JSON.stringify({
            'x': firstXFinal,
            'y': firstYFinal,
            'endX': secondXFinal,
            'endY': secondYFinal
        })

        let deviceURL = `/device/${udid}`

        api.post(deviceURL + '/swipe', jsonData)
            .then(() => { })
            .catch((error) => {
                if (error.response) {
                    if (error.response.status === 404) {
                        showCustomSnackbarError('Swipe failed - Appium session has expired!')
                    } else {
                        showCustomSnackbarError('Swipe failed!')
                    }
                } else {
                    showCustomSnackbarError('Swipe failed!')
                }
            })
    }

    function homeButton() {
        let deviceURL = `/device/${deviceData.udid}/home`

        api.post(deviceURL)
            .then(() => { })
            .catch((error) => {
                if (error.response) {
                    if (error.response.status === 404) {
                        showCustomSnackbarError('Navigation to Home failed - Appium session has expired!')
                    } else {
                        showCustomSnackbarError('Navigation to Home failed!')
                    }
                } else {
                    showCustomSnackbarError('Navigation to Home failed!')
                }
            })
    }

    function lockButton() {
        let deviceURL = `/device/${deviceData.udid}/lock`

        api.post(deviceURL)
            .then(() => { })
            .catch((error) => {
                if (error.response) {
                    if (error.response.status === 404) {
                        showCustomSnackbarError('Device lock failed - Appium session has expired!')
                    } else {
                        showCustomSnackbarError('Device lock failed!')
                    }
                } else {
                    showCustomSnackbarError('Device lock failed!')
                }
            })
    }

    function unlockButton() {
        let deviceURL = `/device/${deviceData.udid}/unlock`

        api.post(deviceURL)
            .then(() => { })
            .catch((error) => {
                if (error.response) {
                    if (error.response.status === 404) {
                        showCustomSnackbarError('Device unlock failed - Appium session has expired!')
                    } else {
                        showCustomSnackbarError('Device unlock failed!')
                    }
                } else {
                    showCustomSnackbarError('Device unlock failed!')
                }
            })
    }
}
