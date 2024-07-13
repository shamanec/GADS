import {Box, Dialog, DialogContent, Grid} from "@mui/material";
import { Button } from "@mui/material";
import { Stack } from "@mui/material";
import { useDialog } from "../../SessionDialogContext";
import { api } from '../../../../services/api.js'
import { useState } from 'react';

export default function Screenshot({ udid }) {
    const { setDialog } = useDialog()
    const [screenshots, setScreenshots] = useState([])
    const [open, setOpen] = useState(false)
    const [selectedImage, setSelectedImage] = useState(null)

    function takeScreenshot() {
        const url = `/device/${udid}/screenshot`
        api.post(url)
            .then(response => {
                if (response.status === 404) {
                    setDialog(true)
                    return
                }
                return response.data
            })
            .then(screenshotJson => {
                const imageBase64String = screenshotJson.value
                setScreenshots(prevScreenshots => [...prevScreenshots, imageBase64String])
            })
            .catch(() => {
            })
    }

    const handleClickOpen = (image) => {
        setSelectedImage(image)
        setOpen(true)
    }

    const handleClose = () => {
        setOpen(false)
    }

    return (
        <Box
            style={{
                marginTop: '20px'
            }}
        >
            <Stack
                style={{
                    height: '800px',
                    width: '100%'
                }}
            >
                <Button
                    onClick={() => takeScreenshot()}
                    variant="contained"
                    style={{
                        marginBottom: '10px',
                        backgroundColor: '#2f3b26',
                        color: '#9ba984',
                        fontWeight: 'bold',
                        width: '200px'
                    }}
                >
                    Take Screenshot
                </Button>
                <Box
                    style={{
                        maxHeight: '800px',
                        overflowY: 'auto'
                    }}
                >
                    <Grid container spacing={2}>
                        {screenshots.map((imageBase64String, index) => (
                            <Grid item key={index}>
                                <img
                                    src={`data:image/png;base64,${imageBase64String}`}
                                    alt={`Screenshot ${index + 1}`}
                                    style={{
                                        maxHeight: '300px',
                                        cursor: 'pointer'
                                    }}
                                    onClick={() => handleClickOpen(`data:image/png;base64,${imageBase64String}`)}
                                />
                            </Grid>
                        ))}
                    </Grid>
                </Box>
            </Stack>
            <Dialog
                open={open}
                onClose={handleClose}
                maxWidth="lg"
            >
                <DialogContent>
                    <img
                        src={selectedImage}
                        alt='Selected Screenshot'
                        style={{
                            width: '100%',
                            height: 'auto'
                        }}
                    />
                </DialogContent>
            </Dialog>
        </Box>
    );
}

