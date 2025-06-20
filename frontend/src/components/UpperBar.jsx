import '../css/UpperBar.css';
import logoFull from '../assets/logo-full-white.png';
import Box from '@mui/material/Box';
import TextField from '@mui/material/TextField';


function Bar({user, handleSignOut}) {
  console.log("User in Bar component:", user);
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
         <Box   
           component="form"
           sx={{ '& > :not(style)': { m: 2, width: '40ch' } }}
           noValidate
           autoComplete="off"
          >
           <TextField id="filled-basic" label="search" variant="filled" />
         </Box>
      </div>
      <div>
        <img src={logoFull} alt="Full White Logo" className="logo" />
      </div>
    </div>
  );
}
export default Bar;
// This component renders a navigation bar with links and a logo.
// The links are placeholders and can be updated to point to actual routes.