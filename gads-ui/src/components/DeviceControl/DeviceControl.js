import { useParams } from 'react-router-dom';
import { useNavigate } from 'react-router-dom';
import { useEffect } from 'react';
import StreamCanvas from './StreamCanvas'

export default function DeviceControl() {
    const { id } = useParams();
    const deviceData = JSON.parse(localStorage.getItem(id))
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
                console.log('component unmounted')
                streamSocket.close()
            }
        }
    }, [])

    return (
        <div>
            <div className='back-button-bar' style={{
                marginBottom: '10px',
                marginTop: '10px'
            }}>
                <BackButton />
            </div>
            <StreamCanvas deviceData={deviceData}></StreamCanvas>
        </div>
    )
}

function BackButton() {
    const navigate = useNavigate();

    const handleBackClick = () => {
        navigate('/devices');
    };

    return (
        <button onClick={handleBackClick}
            style={{
                borderRadius: '5px',
                backgroundColor: 'white',
                border: '1px solid #f3f3f5',
                padding: '5px'
            }}> &larr; Go back to devices</button>
    )
}