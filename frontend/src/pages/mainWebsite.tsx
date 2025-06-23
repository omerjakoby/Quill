import Bar from '../components/UpperBar';  
import SideBar from '../components/SideBar';
import Inbox from '../components/Inbox';
import Unread from '../components/Unread';
import Compose from '../components/Compose';
import Sent from '../components/SentMail';
import '../css/MainWebsite.css';
import { Routes,Route } from 'react-router-dom';
import { User } from 'firebase/auth'; // Importing User type from Firebase Auth


interface MainWebsiteProps {
  user: User | null; // 'user' can be a Firebase User object OR null
  handleSignOut: () => Promise<void>; // 'handleSignOut' is a function returning a Promise that resolves to void
}

function MainWebsite({ user, handleSignOut }: Readonly<MainWebsiteProps>) {
  return (
    <div className="main-website-container">
      <div className="top-bar-container">
        <Bar user={user} handleSignOut={handleSignOut}/>
      </div>
      <div className="content-container">
        <aside className="sidebar-container">
          <SideBar />
        </aside>
        <div className="content-area">
          <Routes>
            <Route path="/" element={<Inbox user={user}/>} />
            <Route path="/Compose" element={<Compose />} />
            <Route path="/Unread" element={<Unread />} />
            <Route path="/Inbox" element={<Inbox user={user}/>} />
            <Route path="/Sent" element={<Sent />} />
          </Routes>
        </div>
     </div>
    </div>
  );
}

export default MainWebsite;