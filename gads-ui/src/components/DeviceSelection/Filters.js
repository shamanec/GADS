import Tabs from '@mui/material/Tabs';
import Tab from '@mui/material/Tab';
import React, { useEffect, useState } from 'react'
import './Filters.css'
import { FiSearch } from "react-icons/fi";

export function OSFilterTabs({ currentTabIndex, handleTabChange }) {
    return (
        <Tabs value={currentTabIndex} onChange={handleTabChange} TabIndicatorProps={{ style: { background: "#282c34", height: "5px" } }} textColor='#f5be4a' sx={{ color: "#f5be4a", fontFamily: "Verdana" }}>
            <Tab label="All" />
            <Tab label="Android" />
            <Tab label="iOS" />
        </Tabs>
    )
}

export function DeviceSearch({ keyUpFilterFunc }) {
    return (
        <div id='search-wrapper'>
            <div id='image-wrapper'>
                <FiSearch size={25} />
            </div>
            <input type="search" id="search-input" onKeyUp={() => keyUpFilterFunc()} placeholder="Search devices"></input>
        </div>
    )
}