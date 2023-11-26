import './TopNavigationBar.css'
import { NavLink } from 'react-router-dom'

export default function NavBar() {
    return (
        <nav className="navbar">
            <div style={{ color: 'white', marginRight: '10px', marginLeft: '100px' }}>GADS</div>
            <StyledNavLink to="/" linkText='Home' />
            <StyledNavLink to="/devices" linkText='Devices' />
            <StyledNavLink to="/logs" linkText='Logs' />
        </nav>
    )
}

function StyledNavLink({ to, linkText }) {
    return (
        <NavLink className="nav-bar-link"
            style={({ isActive }) => ({
                backgroundColor: isActive ? "#c28604" : "",
                color: "#083702",
            })}
            to={to}
        >
            {linkText}
        </NavLink>
    )
}