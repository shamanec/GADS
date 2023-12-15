import React, { useState, useContext } from 'react'
import axios from 'axios'
import { Auth } from '../../contexts/Auth';
import CircularProgress from '@mui/material/CircularProgress';
import { Box, Alert, Button } from '@mui/material';
import './UploadFile.css'
import FileUploadIcon from '@mui/icons-material/FileUpload';
import { List, ListItem, ListItemIcon, ListItemText, ListSubheader } from '@mui/material';
import DescriptionIcon from '@mui/icons-material/Description';
import AttachFileIcon from '@mui/icons-material/AttachFile';


export default function UploadFile({ deviceData }) {
    const [file, setFile] = useState(null);
    const [fileName, setFileName] = useState('No data')
    const [fileSize, setFileSize] = useState('No data')

    // Alert
    const [showAlert, setShowAlert] = useState(false)
    const [alertText, setAlertText] = useState()
    const [alertSeverity, setAlertSeverity] = useState()

    // Upload button
    const [buttonDisabled, setButtonDisabled] = useState(true)

    function handleFileChange(e) {
        if (e.target.files) {
            const targetFile = e.target.files[0]
            const fileExtension = targetFile.name.split('.').pop();

            // If the provided file does not have valid extension
            if (fileExtension != 'apk' && fileExtension != 'ipa' && fileExtension != 'zip') {
                // Still show the selected file name and size
                setFileName(targetFile.name)
                setFileSize((targetFile.size / (1024 * 1024)).toFixed(2) + ' mb')
                // Show an alert and disable the upload button
                setAlertSeverity('error')
                setAlertText('Invalid file extension, only `apk`, `ipa` and `zip` allowed')
                setShowAlert(true)
                setButtonDisabled(true)
                return
            }

            // If the file has a valid extension
            // Enable the button, hide any presented alert and present the file details
            setButtonDisabled(false)
            setShowAlert(false)
            setFileName(targetFile.name)
            setFileSize((targetFile.size / (1024 * 1024)).toFixed(2) + ' mb')
            setFile(targetFile);
        } else {
            return
        }
    }

    return (
        <Box id='upload-wrapper'>
            <h3>Upload app</h3>
            <Button
                component='label'
                variant='contained'
                startIcon={<AttachFileIcon />}
            >
                <input id='file' type="file" onChange={(event) => handleFileChange(event)} style={{ display: 'none' }} />
                Select file
            </Button>
            <List
                subheader={
                    <ListSubheader component="div">
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
                setAlertSeverity={setAlertSeverity}
                setAlertText={setAlertText}
                setShowAlert={setShowAlert}
            ></Uploader>
            {showAlert && <Alert id="add-user-alert" severity={alertSeverity}>{alertText}</Alert>}
        </Box>
    )
}

function Uploader({ file, deviceData, buttonDisabled, setShowAlert, setAlertSeverity, setAlertText }) {
    const [authToken, , logout] = useContext(Auth)
    const [isUploading, setIsUploading] = useState(false)

    function handleUpload() {
        setIsUploading(true)
        const url = `http://${deviceData.host_address}:10001/provider/uploadFile`;

        const form = new FormData();
        form.append('file', file);

        axios.post(url, form, {
            headers: {
                'X-Auth-Token': authToken,
                'Content-Type': 'multipart/form-data'
            }
        })
            .then((response) => {
                setShowAlert(true)
                setAlertSeverity('success')
                setAlertText(response.data.message)
                setIsUploading(false)
            })
            .catch(error => {
                if (error.response) {
                    if (error.response.status === 401) {
                        logout()
                        return
                    }
                    setShowAlert(true)
                    setAlertSeverity('success')
                    setAlertText(error.response.data.message)
                    setIsUploading(false)
                }
                setIsUploading(false)
                setShowAlert(true)
                setAlertSeverity('success')
                setAlertText('Failed uploading file')
                console.log('Failed uploading file - ' + error)
            });
    }

    return (
        <Box id='upload-box'>
            <Button startIcon={<FileUploadIcon />} id='upload-button' variant='contained' onClick={handleUpload} disabled={isUploading || buttonDisabled}>Upload</Button>
            {isUploading &&
                <CircularProgress id='progress-indicator' size={30} />
            }
        </Box>
    )
}
