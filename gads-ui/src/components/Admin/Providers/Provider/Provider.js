import { Stack } from "@mui/material";
import ProviderConfig from "./ProviderConfig";

export default function Provider({ isNew, data }) {
    return (
        <Stack>
            <ProviderConfig isNew={isNew} data={data}>

            </ProviderConfig>
        </Stack>
    )
}