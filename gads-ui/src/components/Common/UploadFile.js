import React, { useRef, useState, useContext } from 'react'
import { Button } from '@mui/material';
import axios from 'axios'
import { Auth } from '../../contexts/Auth';

export default function UploadFile({ deviceData }) {
    const [authToken, , logout] = useContext(Auth)
    const [file, setFile] = useState(null);
    const [fileName, setFileName] = useState(null)
    const [fileSize, setFileSize] = useState(null)

    function handleFileChange(e) {
        if (e.target.files) {
            const targetFile = e.target.files[0]
            setFileName(targetFile.name)
            console.log(targetFile.size)
            setFileSize((targetFile.size / (1024 * 1024)).toFixed(2))
            setFile(e.target.files[0]);
        } else {
            return
        }
    }

    function handleUpload() {
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
                console.log(response)
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
    }

    return (
        < >
            <input id='file' type="file" onChange={(event) => handleFileChange(event)} />
            {file && (
                <div>
                    <section>
                        File details:
                        <ul>
                            <li>Name: {fileName}</li>
                            <li>Size: {fileSize} mb</li>
                        </ul>
                    </section>
                    <Button variant='contained' onClick={handleUpload}>Upload</Button>
                </div>
            )}
        </>
    )
}
