import React, { useState, useContext } from 'react'
import { Button } from '@mui/material';
import axios from 'axios'
import { Auth } from '../../contexts/Auth';
import CircularProgress from '@mui/material/CircularProgress';
import { Box } from '@mui/material';
import './UploadFile.css'
import FileUploadIcon from '@mui/icons-material/FileUpload';
import { List, ListItem, ListItemIcon, ListItemText, ListSubheader } from '@mui/material';
import DescriptionIcon from '@mui/icons-material/Description';
import AttachFileIcon from '@mui/icons-material/AttachFile';

export default function UploadFile({ deviceData }) {
    const [file, setFile] = useState(null);
    const [fileName, setFileName] = useState(null)
    const [fileSize, setFileSize] = useState(null)

    function handleFileChange(e) {
        if (e.target.files) {
            const targetFile = e.target.files[0]
            setFileName(targetFile.name)
            setFileSize((targetFile.size / (1024 * 1024)).toFixed(2))
            setFile(targetFile);
        } else {
            return
        }
    }

    return (
        <Box id='upload-wrapper'>
            <h3>Upload file</h3>
            <Button
                component='label'
                variant='contained'
                startIcon={<AttachFileIcon />}
            >
                <input id='file' type="file" onChange={(event) => handleFileChange(event)} style={{ display: 'none' }} />
                Select file
            </Button>
            {file && (
                <>
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
                                primary={fileSize + ' mb'}
                            />
                        </ListItem>
                    </List>

                    <Uploader file={file} deviceData={deviceData}></Uploader>
                </>
            )}
        </Box>
    )
}

function Uploader({ file, deviceData }) {
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
                console.log(response.data)
            })
            .catch(error => {
                if (error.response) {
                    if (error.response.status === 401) {
                        logout()
                        return
                    }
                    console.log(error.response)
                }
                console.log('Failed uploading file - ' + error)
            });
        setTimeout(() => {
            setIsUploading(false)
        }, 2000)
    }

    return (
        <Box id='upload-box'>
            <Button startIcon={<FileUploadIcon />} id='upload-button' variant='contained' onClick={handleUpload} disabled={isUploading}>Upload</Button>
            {isUploading &&
                <CircularProgress id='progress-indicator' size={30} />

            }
        </Box>
    )
}
