import { NavLink} from 'react-router-dom';
import '../css/SideBar.css'; // Import your CSS for styling
import { Settings, Pencil, Send, Mailbox, Mail, ArrowLeftToLine, ArrowRightToLine } from 'lucide-react';
import logo from '../assets/logo-sidebar.png'; // Import your logo if needed
import { useState } from 'react';

function SideBar() {


  const [isOpen, setIsOpen] = useState(true);

  const toggleSidebar = () => {
    setIsOpen(prev => !prev);
  };

  return (
    <nav className={`sidebar ${isOpen ? 'open' : 'closed'}`}>
      <div className="sidebar-header">
        <img src={logo} alt="Logo" className="sidebar-logo" />
        <button className="sidebar-toggle" onClick={toggleSidebar}>
          {isOpen ? <ArrowLeftToLine size='25' /> : <ArrowRightToLine size='25' />}
        </button>
      </div>
      <ul>
        <li><NavLink to="/Inbox" className={({ isActive }) =>isActive ? 'sidebar-nav-active' : 'sidebar-nav-link'}><Mail size='17'/>  inbox</NavLink></li>
        <li><NavLink to="/Compose" className={({ isActive }) =>isActive ? 'sidebar-nav-active' : 'sidebar-nav-link'} ><Pencil size='17'/>  compose</NavLink></li>
        <li><NavLink to="/Unread" className={({ isActive }) =>isActive ? 'sidebar-nav-active' : 'sidebar-nav-link'}><Mailbox size='17'/>  unread</NavLink></li>    
        <li><NavLink to="/Sent" className={({ isActive }) =>isActive ? 'sidebar-nav-active' : 'sidebar-nav-link'}><Send size='17'/>  sent</NavLink></li>
        <li id="settings"><span id="settingsSpan">settings</span><Settings size='25' /></li>
      </ul>
    </nav>
  );
}   

export default SideBar;