import {Stack} from "@mui/material";
import UploadSeleniumJar from "./UploadSeleniumJar";

export default function FilesAdministration() {
    return(
        <Stack
            style={{
                marginLeft: '20px',
                marginTop: '20px'
            }}
        >
            <UploadSeleniumJar></UploadSeleniumJar>
        </Stack>
    )
}