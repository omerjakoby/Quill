import Bar from '../components/UpperBar';
import Content from '../components/Content';    
import SideBar from '../components/SideBar';
import '../css/MainWebsite.css';
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
          <Content user={user} handleSignOut={handleSignOut} />
        </div>
     </div>
    </div>
  );
}

export default MainWebsite;