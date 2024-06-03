import {Box, Button, CircularProgress, Stack, TextField} from "@mui/material";
import InstallMobileIcon from "@mui/icons-material/InstallMobile";
import {api} from "../../../../services/api";
import {useState} from "react";


export default function Clipboard({ deviceData }) {
    const [isGettingCb, setIsGettingCb] = useState(false)
    const [cbValue, setCbValue] = useState("")

    function handleGetClipboard() {
        setIsGettingCb(true)
        const url = `/device/${deviceData.udid}/getClipboard`
        setCbValue("")
        api.get(url)
            .then((response) => {
                setCbValue(response.data)
                setIsGettingCb(false)
            })
            .catch(() => {
                setIsGettingCb(false)
            });
    }

    return (
        <Box
            marginTop='10px'
            style={{
                backgroundColor: "#9ba984",
                width: "600px"
            }}
        >
            <Stack
                style={{
                    marginLeft: "10px",
                    marginBottom: "10px"
                }}
            >
                <h3>Get clipboard value</h3>
                <Button
                    onClick={handleGetClipboard}
                    id='install-button'
                    variant='contained'
                    disabled={isGettingCb}
                    style={{
                        backgroundColor: isGettingCb ? "rgba(51,71,110,0.47)" : "#2f3b26",
                        color: "#9ba984",
                        fontWeight: "bold"
                    }}
                >Get clipboard</Button>
                {isGettingCb &&
                    <CircularProgress id='progress-indicator' size={30} />
                }
                <TextField
                    id="outlined-basic"
                    variant="outlined"
                    value={cbValue}
                    style={{
                        backgroundColor: '#9ba984',
                        marginTop: '15px',
                        width: '98%'
                    }}
                />
            </Stack>
        </Box>
    )
}