import React, { useEffect, useState } from 'react'
import './DeviceTable.css'

export default function DeviceTable({ name }) {
    let devicesSocket = null;
    const [devices, setDevices] = useState([]);

    useEffect(() => {
        if (devicesSocket) {
            devicesSocket.close()
        }
        devicesSocket = new WebSocket('ws://192.168.1.28:10000/available-devices');

        devicesSocket.onmessage = (message) => {
            let devicesJson = JSON.parse(message.data)

            // Assuming the receivedData is an array of objects
            setDevices(devicesJson);
            devicesJson.forEach((device) => {
                localStorage.setItem(device.udid, JSON.stringify(device))
            })
        }

        return () => {
            if (devicesSocket) {
                devicesSocket.close()
            }
        }
    }, [])

    return (
        <div id="wrapper-top">
            <input type="text" id="search-input" onKeyUp={() => filterDevices()} placeholder="Search devices"></input>
            <p></p>
            <div class="flex-container devices-container" id="devices-container">
                {
                    devices.map((device, index) => {
                        let connected = device.connected.toString()
                        return (
                            <DeviceBox device={device} index={index} key={index} />
                        )
                    })
                }
            </div>
        </div>
    )
}

function DeviceBox({ device, index }) {
    let img_src = device.os === 'android' ? './images/default-android.png' : './images/default-apple.png'
    return (
        <div className='device-box' data-id={device.udid}>
            <div>
                <img className="deviceImage" src={img_src}>
                </img>
            </div>
            <h5 className='filterable'>{device.model}</h5>
            <h6 className='filterable'>{device.os_version}</h6>
            <div className='device-buttons-container'>
                <UseButton device={device} />
                <button className='device-buttons'>Details</button>
            </div>
        </div>
    )
}

function filterDevices() {
    var input = document.getElementById("search-input");
    var filter = input.value.toUpperCase();
    let grid = document.getElementById('devices-container')
    let deviceBoxes = grid.getElementsByClassName('device-box')
    for (let i = 0; i < deviceBoxes.length; i++) {
        let shouldDisplay = false
        var filterables = deviceBoxes[i].getElementsByClassName("filterable")
        for (let j = 0; j < filterables.length; j++) {
            var filterable = filterables[j]
            var txtValue = filterable.textContent || filterable.innerText;
            if (txtValue.toUpperCase().indexOf(filter) > -1) {
                shouldDisplay = true
            }
        }

        if (shouldDisplay) {
            deviceBoxes[i].style.display = "";
        } else {
            deviceBoxes[i].style.display = "none";
        }
    }
}

function UseButton({ device }) {
    const [apiResponse, setApiResponse] = useState(null);
    // Difference between current time and last time the device was reported as healthy
    // let healthyDiff = (Date.now() - device.last_healthy_timestamp)

    if (device.connected === true) {
        return (
            <button className='device-buttons' onClick={() => handleUseButtonClick({ device })}>Use</button>
        )
    } else {
        return (
            <button className='device-buttons' disabled>N/A</button>
        )
    }

    async function handleUseButtonClick({ device }) {
        const url = "http://" + device.host_address + ":10000/device/" + device.udid + "/health"
        const response = await fetch(url);
        setApiResponse(response);
    }
}
