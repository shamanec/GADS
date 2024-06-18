import React, { useContext, useState } from 'react'
import './DeviceSelection.css'
import { useNavigate } from 'react-router-dom';
import './Device.css'
import { Auth } from '../../contexts/Auth';
import { api } from '../../services/api.js'

export function DeviceBox({ device, handleAlert }) {
    let img_src = device.info.os === 'android' ? './images/default-android.png' : './images/default-apple.png'

    return (
        <div
            className='device-box'
            data-id={device.info.udid}
        >
            <div>
                <img
                    className="deviceImage"
                    src={img_src}
                >
                </img>
            </div>
            <div
                className='filterable info'
                style={{ fontSize: "16px"}}
            >{device.info.name}</div>
            <div
                className='filterable info'
                style={{ fontSize: "16px"}}
            >{device.info.os_version}</div>
            <div
                className='device-buttons-container'
            >
                <UseButton device={device} handleAlert={handleAlert} />
            </div>
        </div>
    )
}

function UseButton({ device, handleAlert }) {
    // Difference between current time and last time the device was reported as healthy
    // let healthyDiff = (Date.now() - device.last_healthy_timestamp)
    const [loading, setLoading] = useState(false)

    const navigate = useNavigate();

    function handleUseButtonClick() {
        setLoading(true);
        const url = `/device/${device.info.udid}/health`;
        api.get(url)
            .then(response => {
                if (response.status === 200) {
                    navigate('/devices/control/' + device.info.udid, device);
                }
            })
            .catch(() => {
                handleAlert()
            })
            .finally(() => {
                setTimeout(() => {
                    setLoading(false);
                }, 2000);
            });
    }

    const buttonDisabled = loading || !device.info.connected;

    if (device.in_use === true) {
        return (
            <button className='device-buttons in-use' disabled>In Use</button>
        )
    } else if (device.info.available === true) {
        return (
            <button className='device-buttons' onClick={handleUseButtonClick} disabled={buttonDisabled}>
                {loading ? <span className="spinner"></span> : 'Use'}
            </button>
        );
    } else {
        return (
            <button className='device-buttons' disabled>N/A</button>
        );
    }
}