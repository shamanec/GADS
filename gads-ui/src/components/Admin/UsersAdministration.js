import AddUser from "./AddUser"
import Stack from '@mui/material/Stack';

export default function UsersAdministration() {
    return (
        <Stack direction="row" spacing={2}>
            <AddUser></AddUser>
        </Stack>
    )
}