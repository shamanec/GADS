import React, { useContext, useEffect, useState } from 'react'
import './DeviceSelection.css'
import { useNavigate } from 'react-router-dom';
import './Device.css'
import { Auth } from '../../contexts/Auth';

export function DeviceBox({ device, handleAlert }) {
    let img_src = device.os === 'android' ? './images/default-android.png' : './images/default-apple.png'

    return (
        <div className='device-box' data-id={device.udid}>
            <div>
                <img className="deviceImage" src={img_src}>
                </img>
            </div>
            <div className='filterable info'>{device.name}</div>
            <div className='filterable info'>{device.os_version}</div>
            <div className='device-buttons-container'>
                <UseButton device={device} handleAlert={handleAlert} />
            </div>
        </div>
    )
}

function UseButton({ device, handleAlert }) {
    // Difference between current time and last time the device was reported as healthy
    // let healthyDiff = (Date.now() - device.last_healthy_timestamp)
    const [loading, setLoading] = useState(false);
    const [authToken, username, , login, logout] = useContext(Auth)

    const navigate = useNavigate();

    function handleUseButtonClick() {
        setLoading(true);
        const url = `/device/${device.udid}/health`;
        fetch(url, {
            headers: {
                'X-Auth-Token': authToken
            }
        })
            .then((response) => {
                console.log(response.status)
                if (!response.ok) {
                    if (response.status === 401) {
                        logout()
                        return
                    }
                    throw new Error('Network response was not ok');
                } else {
                    navigate('/devices/control/' + device.udid, device);
                }
            })
            .catch((error) => {
                handleAlert()
                console.error('Error fetching data:', error);

            })
            .finally(() => {
                setTimeout(() => {
                    setLoading(false);
                }, 2000);
            });
    }

    const buttonDisabled = loading || !device.connected;

    if (device.in_use === true) {
        return (
            <button className='device-buttons in-use' disabled>In Use</button>
        )
    } else if (device.connected === true) {
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