import UploadFile from "../../Common/UploadFile";
import { Box } from "@mui/material";

export default function Actions({ deviceData }) {
    return (
        <Box marginTop='10px'>
            <UploadFile deviceData={deviceData}></UploadFile>
        </Box>
    )
}