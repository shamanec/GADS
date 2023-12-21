import Apps from "./Apps/Apps";
import { Box, Stack } from "@mui/material";

export default function Actions({ deviceData }) {
    return (
        <Box marginTop='10px'>
            <Stack>
                <Apps deviceData={deviceData}>

                </Apps>
            </Stack>
        </Box>
    )
}