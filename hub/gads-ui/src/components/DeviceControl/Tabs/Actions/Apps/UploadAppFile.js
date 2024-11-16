import React, { useState } from 'react'
import CircularProgress from '@mui/material/CircularProgress'
import { Box, Button } from '@mui/material'
import './UploadAppFile.css'
import FileUploadIcon from '@mui/icons-material/FileUpload'
import { List, ListItem, ListItemIcon, ListItemText, ListSubheader } from '@mui/material'
import DescriptionIcon from '@mui/icons-material/Description'
import AttachFileIcon from '@mui/icons-material/AttachFile'
import { api } from '../../../../../services/api.js'
import { useSnackbar } from '../../../../../contexts/SnackBarContext.js'


export default function UploadAppFile({ deviceData }) {
    const { showSnackbar } = useSnackbar()
    // Upload file and file data
    const [file, setFile] = useState(null)
    const [fileName, setFileName] = useState('No data')
    const [fileSize, setFileSize] = useState('No data')

    // Upload button
    const [buttonDisabled, setButtonDisabled] = useState(true)

    function handleFileChange(e) {
        if (e.target.files) {
            const targetFile = e.target.files[0]
            const fileExtension = targetFile.name.split('.').pop()

            // If the provided file does not have valid extension
            if (fileExtension != 'apk' && fileExtension != 'ipa' && fileExtension != 'zip') {
                // Still show the selected file name and size
                setFileName(targetFile.name)
                setFileSize((targetFile.size / (1024 * 1024)).toFixed(2) + ' mb')
                // Disable the upload button
                showCustomSnackbarError('File should be .apk or .ipa or .zip!', 5000)
                setButtonDisabled(true)
                return
            }

            // If the file has a valid extension
            // Enable the button and present the file details
            setButtonDisabled(false)
            setFileName(targetFile.name)
            setFileSize((targetFile.size / (1024 * 1024)).toFixed(2) + ' mb')
            setFile(targetFile)
        } else {
            return
        }
    }

    const showCustomSnackbarError = (message, timeout = 3000) => {
        showSnackbar({
            message: message,
            severity: 'error',
            duration: timeout,
        })
    }

    function Uploader({ file, deviceData, buttonDisabled }) {
        const [isUploading, setIsUploading] = useState(false)

        function handleUpload() {
            setIsUploading(true)
            const url = `/device/${deviceData.udid}/uploadAndInstallApp`

            const form = new FormData()
            form.append('file', file)

            api.post(url, form, {
                headers: {
                    'Content-Type': 'multipart/form-data'
                }
            })
                .then(() => {
                    setIsUploading(false)
                })
                .catch(error => {
                    if (error.response) {
                        setIsUploading(false)
                        showCustomSnackbarError(`Failed to upload '${file}'`)
                    }
                    showCustomSnackbarError(`Failed to upload '${file}'`)
                    setIsUploading(false)
                })
        }

        return (
            <Box id='upload-box'>
                <Button
                    startIcon={<FileUploadIcon />}
                    id='upload-button'
                    variant='contained'
                    onClick={handleUpload}
                    disabled={isUploading || buttonDisabled}
                    style={{
                        backgroundColor: (isUploading || buttonDisabled) ? 'rgba(51,71,110,0.47)' : '#2f3b26',
                        color: '#9ba984',
                        fontWeight: 'bold',
                        width: '250px'
                    }}
                >Upload and install</Button>
                {isUploading &&
                    <CircularProgress id='progress-indicator' size={30} />
                }
            </Box>
        )
    }

    return (
        <Box id='upload-wrapper'>
            <h3>Upload and install app</h3>
            <Button
                component='label'
                variant='contained'
                startIcon={<AttachFileIcon />}
                style={{
                    backgroundColor: '#2f3b26',
                    color: '#9ba984',
                    fontWeight: 'bold'
                }}
            >
                <input
                    id='input-file'
                    type='file'
                    onChange={(event) => handleFileChange(event)}
                />
                Select file
            </Button>
            <List
                subheader={
                    <ListSubheader
                        component='div'
                        style={{
                            backgroundColor: '#9ba984'
                        }}
                    >
                        File details
                    </ListSubheader>
                }
                dense={true}
                alignitems='left'
            >
                <ListItem>
                    <ListItemIcon>
                        <DescriptionIcon />
                    </ListItemIcon>
                    <ListItemText
                        primary={fileName}
                    />
                </ListItem>
                <ListItem>
                    <ListItemIcon>
                        <DescriptionIcon />
                    </ListItemIcon>
                    <ListItemText
                        primary={fileSize}
                    />
                </ListItem>
            </List>
            <Uploader
                file={file}
                deviceData={deviceData}
                buttonDisabled={buttonDisabled}
            ></Uploader>
        </Box>
    )
}
