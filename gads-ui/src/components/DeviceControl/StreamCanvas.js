import { useEffect } from "react"

export default function StreamCanvas({ deviceData }) {
    let screenDimensions = deviceData.screen_size.split("x")
    let deviceX = parseInt(screenDimensions[0], 10)
    let deviceY = parseInt(screenDimensions[1], 10)
    let screen_ratio = deviceX / deviceY
    let canvasHeight = 850
    let canvasWidth = 850 * screen_ratio

    const streamData = {
        udid: deviceData.udid,
        deviceX: deviceX,
        deviceY: deviceY,
        screen_ratio: screen_ratio,
        canvasHeight: canvasHeight,
        canvasWidth: canvasWidth
    }

    let streamSocket = null;
    useEffect(() => {
        if (streamSocket) {
            streamSocket.close()
        }

        if (deviceData.os === 'ios') {
            streamSocket = new WebSocket(`ws://${process.env.REACT_APP_GADS_BACKEND_HOST}/device/${deviceData.udid}/ios-stream`);

            streamSocket.onmessage = (message) => {
                let imgElement = document.getElementById('image-stream')

                streamSocket.onmessage = function (event) {
                    const imageURL = URL.createObjectURL(event.data);
                    imgElement.src = imageURL

                    imgElement.onload = () => {
                        URL.revokeObjectURL(imageURL);
                    };
                }
            }
        } else {
            streamSocket = new WebSocket(`ws://${process.env.REACT_APP_GADS_BACKEND_HOST}/device/${deviceData.udid}/android-stream`);
            streamSocket.binaryType = 'arraybuffer'

            let imgElement = document.getElementById('image-stream')
            streamSocket.onmessage = function (event) {
                // Get the message data
                const data = event.data
                // Get the first 4 bytes of the message to Int
                // To determing the message type - info or image
                const messageType = new DataView(data.slice(0, 4)).getInt32(0, false)

                // If message type is 2(Image)
                if (messageType == 2) {
                    // Create an image URL
                    const imageURL = URL.createObjectURL(new Blob([data.slice(4)]))
                    // Set the image in the image element to create a stream
                    imgElement.src = imageURL

                    imgElement.onload = () => {
                        URL.revokeObjectURL(imageURL);
                    };
                }
            }
        }

        // If component unmounts close the websocket connection
        return () => {
            if (streamSocket) {
                console.log('stream unmounted')
                streamSocket.close()
            }
        }
    }, [])

    return (
        <div id="stream-div">
            <Canvas canvasWidth={canvasWidth} canvasHeight={canvasHeight} streamData={streamData}></Canvas>
            <Stream canvasWidth={canvasWidth} canvasHeight={canvasHeight}></Stream>
        </div>
    )
}

function Canvas({ streamData }) {
    var tapStartAt = 0
    var coord1;
    var coord2;

    function getCursorCoordinates(event) {
        const rect = event.currentTarget.getBoundingClientRect()
        const x = event.clientX - rect.left
        const y = event.clientY - rect.top
        return [x, y];
    }

    function handleMouseDown(event) {
        tapStartAt = (new Date()).getTime()
        coord1 = getCursorCoordinates(event)
        console.log('Tapped on ' + coord1)
    }

    function handleMouseUp(event) {
        coord2 = getCursorCoordinates(event)
        console.log('Released on' + coord2)
        // get the time of finishing the click on the canvas
        var tapEndAt = (new Date()).getTime()

        var mouseEventsTimeDiff = tapEndAt - tapStartAt

        // if the difference of time between click down and click up is more than 500ms assume it is a swipe, not a tap
        // to allow flick swipes we also check the difference between the gesture coordinates
        // x1, y1 = mousedown coordinates
        // x2, y2 = mouseup coordinates
        // if x2 > x1*1.1 - it is probably a swipe left to right
        // if x2 < x1*0.9 - it is probably a swipe right to left
        // if y2 < y1*0.9 - it is probably a swipe bottom to top
        // if y2 > y1*1.1 - it is probably a swipe top to bottom
        if (mouseEventsTimeDiff > 500 || coord2[0] > coord1[0] * 1.1 || coord2[0] < coord1[0] * 0.9 || coord2[1] < coord1[1] * 0.9 || coord2[1] > coord1[1] * 1.1) {
            swipeCoordinates(coord1, coord2, streamData)
        } else {
            tapCoordinates(coord1, streamData)
        }
    }

    return (
        <canvas id="actions-canvas" onMouseDown={handleMouseDown} onMouseUp={handleMouseUp} style={{ position: "absolute", width: streamData.canvasWidth + 'px', height: streamData.canvasHeight + 'px' }}></canvas>
    )
}

function Stream({ canvasWidth, canvasHeight }) {
    return (
        <img id="image-stream" style={{ width: canvasWidth + 'px', height: canvasHeight + 'px' }}></img>
    )
}

// tap with WDA using coordinates
function tapCoordinates(pos, streamData) {
    // set initial x and y tap coordinates
    let x = pos[0]
    let y = pos[1]

    // if the stream height 
    if (streamData.canvasHeight != streamData.deviceY) {
        x = (x / streamData.canvasWidth) * streamData.deviceX
        y = (y / streamData.canvasHeight) * streamData.deviceY
    }

    let jsonData = JSON.stringify({
        "x": x,
        "y": y
    })

    let url = `http://${process.env.REACT_APP_GADS_BACKEND_HOST}/device/${streamData.udid}/tap`

    fetch(url, {
        method: 'POST',
        body: jsonData,
        headers: {
            'Content-type': 'application/json'
        }
    })
        .then(response => {
            if (response.status === 404) {
                console.log('fail')
                return
            }
        })
        .catch(function (error) {
            alert("Could not access device homescreen endpoint. Error: " + error)
        })
}

function swipeCoordinates(coord1, coord2, streamData) {

    var firstCoordX = coord1[0]
    var firstCoordY = coord1[1]
    var secondCoordX = coord2[0]
    var secondCoordY = coord2[1]

    // if the stream height 
    if (streamData.canvasHeight != streamData.deviceY) {
        firstCoordX = (firstCoordX / streamData.canvasWidth) * streamData.deviceX
        firstCoordY = (firstCoordY / streamData.canvasHeight) * streamData.deviceY
        secondCoordX = (secondCoordX / streamData.canvasWidth) * streamData.deviceX
        secondCoordY = (secondCoordY / streamData.canvasHeight) * streamData.deviceY
    }

    let jsonData = JSON.stringify({
        "x": firstCoordX,
        "y": firstCoordY,
        "endX": secondCoordX,
        "endY": secondCoordY
    })

    let url = `http://${process.env.REACT_APP_GADS_BACKEND_HOST}/device/${streamData.udid}/swipe`

    fetch(url, {
        method: 'POST',
        body: jsonData,
        headers: {
            'Content-type': 'application/json'
        }
    })
        .then(response => {
            if (response.status === 404) {
                return
            }
        })
        .catch(function (error) {
            alert("Could not access device homescreen endpoint. Error: " + error)
        })

}