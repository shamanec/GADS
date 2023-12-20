import Stack from '@mui/material/Stack';
import { FaHome } from "react-icons/fa";
import { FaLock, FaUnlock, FaEraser } from "react-icons/fa";
import './ActionsStack.css'
import ShowFailedSessionAlert from './SessionAlert';
import { Auth } from '../../contexts/Auth';
import { useContext } from 'react';

export default function ActionsStack({ deviceData }) {
    const [authToken, login, logout] = useContext(Auth)

    let deviceURL = `/device/${deviceData.Device.udid}`
    return (
        <Stack spacing={0} style={{ width: "50px" }}>
            <HomeButton authToken={authToken} deviceURL={deviceURL}></HomeButton>
            <LockButton authToken={authToken} deviceURL={deviceURL}></LockButton>
            <UnlockButton authToken={authToken} deviceURL={deviceURL}></UnlockButton>
            <ClearTextButton authToken={authToken} deviceURL={deviceURL}></ClearTextButton>
        </Stack>
    )
}

function HomeButton({ authToken, deviceURL }) {
    function handleClick(deviceURL) {
        let url = `${deviceURL}/home`

        fetch(url, {
            method: 'POST',
            headers: {
                'X-Auth-Token': authToken
            }
        })
            .then(response => {
                if (response.status === 404) {
                    ShowFailedSessionAlert(deviceURL)
                    return
                }
            })
            .catch(() => {
                ShowFailedSessionAlert(deviceURL)
            })
    }

    return (
        <button className='action-buttons' onClick={() => handleClick(deviceURL)}>
            <FaHome size={30} />
        </button>
    )
}

function LockButton({ authToken, deviceURL }) {
    function handleClick(deviceURL) {
        let url = `${deviceURL}/lock`

        fetch(url, {
            method: 'POST',
            headers: {
                'X-Auth-Token': authToken
            }
        })
            .then(response => {
                if (response.status === 404) {
                    ShowFailedSessionAlert(deviceURL)
                    return
                }
            })
            .catch(() => {
                ShowFailedSessionAlert(deviceURL)
            })
    }

    return (
        <button className='action-buttons' onClick={() => handleClick(deviceURL)}>
            <FaLock size={30} />
        </button>
    )
}

function UnlockButton({ authToken, deviceURL }) {
    function handleClick(deviceURL) {
        let url = `${deviceURL}/unlock`

        fetch(url, {
            method: 'POST',
            headers: {
                'X-Auth-Token': authToken
            }
        })
            .then(response => {
                if (response.status === 404) {
                    ShowFailedSessionAlert(deviceURL)
                    return
                }
            })
            .catch(() => {
                ShowFailedSessionAlert(deviceURL)
            })
    }

    return (
        <button className='action-buttons' onClick={() => handleClick(deviceURL)}>
            <FaUnlock size={30} />
        </button>
    )
}

function ClearTextButton({ authToken, deviceURL }) {
    function handleClick(deviceURL) {
        let url = `${deviceURL}/clearText`

        fetch(url, {
            method: 'POST',
            headers: {
                'X-Auth-Token': authToken
            }
        })
            .then(response => {
                if (response.status === 404) {
                    ShowFailedSessionAlert(deviceURL)
                    return
                }
            })
            .catch(() => {
                ShowFailedSessionAlert(deviceURL)
            })
    }

    return (
        <button className='action-buttons' onClick={() => handleClick(deviceURL)}>
            <FaEraser size={30} />
        </button>
    )
}