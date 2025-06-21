import Bar from '../components/UpperBar';
import Content from '../components/Content';    
import SideBar from '../components/SideBar';
import Inbox from '../components/Inbox';
import Unread from '../components/Unread';
import Compose from '../components/Compose';
import Sent from '../components/SentMail';
import '../css/MainWebsite.css';
import { Routes,Route } from 'react-router-dom';




function MainWebsite({ user, handleSignOut }) {
  return (
    <div className="main-website-container">
      <div className="top-bar-container">
        <Bar user={user} handleSignOut={handleSignOut}/>
      </div>
      <div className="content-container">
        <div className="sidebar-container">
          <SideBar />
        </div>
        <div className="content-area">
          <Routes>
            <Route path="/" element={<Inbox/>} />
            <Route path="/Compose" element={<Compose/>} />
            <Route path="/Unread" element={<Unread user={user} handleSignOut={handleSignOut}/>} />
            <Route path="/Inbox" element={<Inbox />} />
            <Route path="/Sent" element={<Sent />} />
          </Routes>
        </div>
     </div>
    </div>
  );
}

export default MainWebsite;