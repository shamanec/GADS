import Tabs from '@mui/material/Tabs'
import Tab from '@mui/material/Tab'
import React, { useEffect, useState } from 'react'
import './Filters.css'
import { FiSearch } from 'react-icons/fi'
import { Box, FormControl, Select, MenuItem } from '@mui/material'
import { api } from '../../services/api'

export function OSFilterTabs({ currentTabIndex, handleTabChange }) {
    return (
        <Tabs
            value={currentTabIndex}
            onChange={handleTabChange}
            TabIndicatorProps={{
                style: {
                    background: '#2f3b26',
                    height: '5px'
                }
            }}
            textColor='#f4e6cd'
            sx={{
                color: '#2f3b26',
                fontFamily: 'Verdana'
            }}
        >
            <Tab label='All' />
            <Tab label='Android' />
            <Tab label='iOS' />
        </Tabs>
    )
}

export function DeviceSearch({ keyUpFilterFunc }) {
    return (
        <div id='search-wrapper'>
            <div id='image-wrapper'>
                <FiSearch size={25} />
            </div>
            <input
                type='search'
                id='search-input'
                onInput={() => keyUpFilterFunc()}
                placeholder='Search devices'
                className='custom-placeholder'
                autoComplete='off'
            ></input>
        </div>
    )
}

export function WorkspaceSelector({ selectedWorkspace, setSelectedWorkspace }) {
    const [workspaces, setWorkspaces] = useState([]);

    useEffect(() => {
        fetchWorkspaces();
    }, []);

    const fetchWorkspaces = async () => {
        try {
            const response = await api.get('/workspaces?page=1&limit=10&search=');
            setWorkspaces(response.data.workspaces);
            
            // Find the default workspace
            const defaultWorkspace = response.data.workspaces.find(ws => ws.is_default);
            if (defaultWorkspace) {
                setSelectedWorkspace(defaultWorkspace.id);
            } else if (response.data.workspaces.length > 0) {
                setSelectedWorkspace(response.data.workspaces[0].id);
            }
        } catch (error) {
            console.error('Failed to fetch workspaces:', error);
        }
    };

    return (
        <Box sx={{ minWidth: 200, padding: '0 10px' }}>
            <FormControl fullWidth size="small">
                <Select
                    value={selectedWorkspace}
                    onChange={(e) => setSelectedWorkspace(e.target.value)}
                    sx={{
                        backgroundColor: '#ffffff',
                        borderRadius: '4px',
                        '& .MuiOutlinedInput-notchedOutline': {
                            borderColor: '#2f3b26',
                        },
                    }}
                >
                    {workspaces.map((workspace) => (
                        <MenuItem key={workspace.id} value={workspace.id}>
                            {workspace.name}
                        </MenuItem>
                    ))}
                </Select>
            </FormControl>
        </Box>
    );
}