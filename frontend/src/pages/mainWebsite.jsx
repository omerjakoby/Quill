import Bar from '../components/UpperBar';
import Content from '../components/Content';    
import SideBar from '../components/SideBar';
import Demo from '../components/demo';
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
            <Route path="/" element={<Content user={user} handleSignOut={handleSignOut} />} />
            <Route path="/Demo" element={<Demo user={user} handleSignOut={handleSignOut} />} />
          </Routes>
        </div>
     </div>
    </div>
  );
}

export default MainWebsite;