import '../css/UpperBar.css';
import logoFull from '../assets/logo-full-white.png';

function Bar({user, handleSignOut}) {
  return (
    <div className="block">
      <div className="main-account-container">
        <div className="user-info">
          <span>Welcome {user.displayName}!</span>
          <button onClick={handleSignOut}>Sign Out</button>
          <img
            src={user.photoURL}
            alt="User profile"
            className="profile-pic"
          />
        </div>         
      </div>
      <div className="bar">
        <button className="barherf">Mail</button>
        <button className="barherf">contacts</button>
        <button className="barherf">Calendar</button>
      </div>
      <div>
        <img src={logoFull} className="logo" />
      </div>
    </div>
  );
}
export default Bar;
// This component renders a navigation bar with links and a logo.
// The links are placeholders and can be updated to point to actual routes.