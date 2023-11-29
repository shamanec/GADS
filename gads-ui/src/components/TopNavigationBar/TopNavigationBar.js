import './TopNavigationBar.css'
import { NavLink } from 'react-router-dom'

export default function NavBar() {
    return (
        <nav className="navbar">
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
                backgroundColor: isActive ? "#273616" : "",
                color: "#E0D8C0",
            })}
            to={to}
        >
            {linkText}
        </NavLink>
    )
}