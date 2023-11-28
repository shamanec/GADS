import Stack from '@mui/material/Stack';
import { FaHome } from "react-icons/fa";
import { FaLock, FaUnlock, FaEraser } from "react-icons/fa";
import './ActionsStack.css'

export default function ActionsStack({ deviceData }) {
    let deviceURL = `http://${process.env.REACT_APP_GADS_BACKEND_HOST}/device/${deviceData.udid}`
    return (
        <Stack spacing={0} style={{ width: "50px" }}>
            <HomeButton deviceURL={deviceURL}></HomeButton>
            <LockButton deviceURL={deviceURL}></LockButton>
            <UnlockButton deviceURL={deviceURL}></UnlockButton>
            <ClearTextButton deviceURL={deviceURL}></ClearTextButton>
        </Stack>
    )
}

function HomeButton({ deviceURL }) {
    function handleClick(deviceURL) {
        let url = `${deviceURL}/home`

        fetch(url, {
            method: 'POST'
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

    return (
        <button className='action-buttons' onClick={() => handleClick(deviceURL)}>
            <FaHome size={30} />
        </button>
    )
}

function LockButton({ deviceURL }) {
    function handleClick(deviceURL) {
        let url = `${deviceURL}/lock`

        fetch(url, {
            method: 'POST'
        })
            .then(response => {
                if (response.status === 404) {
                    return
                }
            })
            .catch(function (error) {
                alert("Could not access device lock endpoint. Error: " + error)
            })
    }

    return (
        <button className='action-buttons' onClick={() => handleClick(deviceURL)}>
            <FaLock size={30} />
        </button>
    )
}

function UnlockButton({ deviceURL }) {
    function handleClick(deviceURL) {
        let url = `${deviceURL}/unlock`

        fetch(url, {
            method: 'POST'
        })
            .then(response => {
                if (response.status === 404) {
                    return
                }
            })
            .catch(function (error) {
                alert("Could not access device lock endpoint. Error: " + error)
            })
    }

    return (
        <button className='action-buttons' onClick={() => handleClick(deviceURL)}>
            <FaUnlock size={30} />
        </button>
    )
}

function ClearTextButton({ deviceURL }) {
    function handleClick(deviceURL) {
        let url = `${deviceURL}/clearText`

        fetch(url, {
            method: 'POST'
        })
            .then(response => {
                if (response.status === 404) {
                    return
                }
            })
            .catch(function (error) {
                alert("Could not access device lock endpoint. Error: " + error)
            })
    }

    return (
        <button className='action-buttons' onClick={() => handleClick(deviceURL)}>
            <FaEraser size={30} />
        </button>
    )
}