import { NavLink,Link } from 'react-router-dom';
import '../css/SideBar.css'; // Import your CSS for styling
import { Settings, Pencil, Send, Mailbox, Mail } from 'lucide-react';

function SideBar() {
  return (
    <div className="sidebar">
      <ul>
        <li><NavLink to="/Inbox" className={({ isActive }) =>isActive ? 'sidebar-nav-active' : 'sidebar-nav-link'}><Mail size='17'/>  inbox</NavLink></li>
        <li><NavLink to="/Compose" className={({ isActive }) =>isActive ? 'sidebar-nav-active' : 'sidebar-nav-link'} ><Pencil size='17'/>  compose</NavLink></li>
        <li><NavLink to="/Unread" className={({ isActive }) =>isActive ? 'sidebar-nav-active' : 'sidebar-nav-link'}><Mailbox size='17'/>  unread</NavLink></li>    
        <li><NavLink to="/Sent" className={({ isActive }) =>isActive ? 'sidebar-nav-active' : 'sidebar-nav-link'}><Send size='17'/>  sent</NavLink></li>
        <li id="settings"><span id="settingsSpan">settings</span><Settings size='20' /></li>
      </ul>
    </div>
  );
}   

export default SideBar;