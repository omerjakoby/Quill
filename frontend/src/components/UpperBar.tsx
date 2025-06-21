import '../css/UpperBar.css';
import logoFull from '../assets/logo-full-white.png';
import Box from '@mui/material/Box';
import TextField from '@mui/material/TextField';

import { User } from 'firebase/auth'; // Importing User type from Firebase Auth


interface TopBarProps {
  user: User | null; // 'user' can be a Firebase User object OR null
  handleSignOut: () => Promise<void>;
}

function Bar({user, handleSignOut}: TopBarProps) {
  console.log("User in Bar component:", user);
  return (
    <div className="block">
      <div className="main-account-container">
        <div className="user-info">        
          <img        
            src={user?.photoURL?? undefined}
            alt="User profile"
            className="profile-pic"
          />
          <button onClick={handleSignOut}>Sign Out</button>
          
        </div>         
      </div>
      <div className="search-bar">
         <Box   
           component="form"
           sx={{ '& > :not(style)': { m: 2, width: '60ch' } }}
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