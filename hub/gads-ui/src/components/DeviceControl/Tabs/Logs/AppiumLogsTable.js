import { useContext, useState } from "react";
import {Auth} from "../../../../contexts/Auth";
import {
    Box,
    Button,
    IconButton,
    MenuItem,
    Paper,
    Table,
    TableBody, TableCell,
    TableContainer, TableFooter, TablePagination,
    TableRow,
    useTheme
} from "@mui/material";
import FirstPageIcon from '@mui/icons-material/FirstPage';
import KeyboardArrowLeft from '@mui/icons-material/KeyboardArrowLeft';
import KeyboardArrowRight from '@mui/icons-material/KeyboardArrowRight';
import LastPageIcon from '@mui/icons-material/LastPage';
import { api } from '../../../../services/api.js'

function TablePaginationActions(props) {
    const theme = useTheme();
    const { count, page, rowsPerPage, onPageChange } = props;

    const handleFirstPageButtonClick = (event) => {
        onPageChange(event, 0);
    };

    const handleBackButtonClick = (event) => {
        onPageChange(event, page - 1);
    };

    const handleNextButtonClick = (event) => {
        onPageChange(event, page + 1);
    };

    const handleLastPageButtonClick = (event) => {
        onPageChange(event, Math.max(0, Math.ceil(count / rowsPerPage) - 1));
    };

    return (
        <Box sx={{ flexShrink: 0, ml: 2.5 }}>
            <IconButton
                onClick={handleFirstPageButtonClick}
                disabled={page === 0}
                aria-label="first page"
            >
                {theme.direction === 'rtl' ? <LastPageIcon /> : <FirstPageIcon />}
            </IconButton>
            <IconButton
                onClick={handleBackButtonClick}
                disabled={page === 0}
                aria-label="previous page"
            >
                {theme.direction === 'rtl' ? <KeyboardArrowRight /> : <KeyboardArrowLeft />}
            </IconButton>
            <IconButton
                onClick={handleNextButtonClick}
                disabled={page >= Math.ceil(count / rowsPerPage) - 1}
                aria-label="next page"
            >
                {theme.direction === 'rtl' ? <KeyboardArrowLeft /> : <KeyboardArrowRight />}
            </IconButton>
            <IconButton
                onClick={handleLastPageButtonClick}
                disabled={page >= Math.ceil(count / rowsPerPage) - 1}
                aria-label="last page"
            >
                {theme.direction === 'rtl' ? <FirstPageIcon /> : <LastPageIcon />}
            </IconButton>
        </Box>
    );
}

export default function AppiumLogsTable({ udid }) {
    const [page, setPage] = useState(0);
    const {logout} = useContext(Auth)
    const [logData, setLogData] = useState([])
    const rowsPerPage = 15

    // Avoid a layout jump when reaching the last page with empty rows.
    const emptyRows =
        page > 0 ? Math.max(0, (1 + page) * rowsPerPage - logData.length) : 0;

    const handleChangePage = (event, newPage) => {
        setPage(newPage);
    };

    function getLogs() {
        const url = `/appium-logs?collection=${udid}`

        api.get(url)
            .then(response => {
                setLogData(response.data)
            })
            .catch(error => {
                if (error.response) {
                    if (error.response.status === 401) {
                        logout()
                        return
                    }
                }
                console.log('Failed getting providers data' + error)
            });
    }

    return (
        <div style={{width: '1200px', marginTop: '10px'}}>
            <Button
                onClick={getLogs}
                id='install-button'
                variant='contained'
                style={{
                    backgroundColor: "#2f3b26",
                    color: "#9ba984",
                    fontWeight: "bold"
                }}
            >Get Logs</Button>
            <TableContainer
                component={Paper}
                style={{
                    marginTop: '10px',
                    backgroundColor: "#9ba984"
                }}
            >
                <Table sx={{ minWidth: 500 }} size='small' padding='checkbox'>
                    <TableBody>
                        {(rowsPerPage > 0
                                ? logData.slice(page * rowsPerPage, page * rowsPerPage + rowsPerPage)
                                : logData
                        ).map((logEntry, index) => (
                            <TableRow key={index}>
                                <TableCell style={{ width: 80, fontSize: "14px" }} align='center'>
                                    {logEntry.appium_ts}
                                </TableCell>
                                <TableCell style={{ width: 200, fontSize: "14px" }}>
                                    {logEntry.log_type}
                                </TableCell>
                                <TableCell style={{ width: 660, maxWidth: 660, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', fontSize: "14px" }}>
                                    {logEntry.msg}
                                </TableCell>
                            </TableRow>
                        ))}
                        {emptyRows > 0 && (
                            <TableRow style={{ height: 40 * emptyRows }}>
                                <TableCell colSpan={6} />
                            </TableRow>
                        )}
                    </TableBody>
                    <TableFooter>
                        <TableRow>
                            <TablePagination
                                rowsPerPageOptions={[]}
                                colSpan={3}
                                count={logData.length}
                                rowsPerPage={rowsPerPage}
                                page={page}
                                slotProps={{
                                    select: {
                                        inputProps: {
                                            'aria-label': 'rows per page',
                                        },
                                        native: true,
                                    },
                                }}
                                onPageChange={handleChangePage}
                                ActionsComponent={TablePaginationActions}
                            />
                        </TableRow>
                    </TableFooter>
                </Table>
            </TableContainer>
        </div>
    );
}