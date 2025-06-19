import Bar from '../components/UpperBar';
import Content from '../components/Content';    
function MainWebsite({ user, handleSignOut }) {
  return (
    <>
        <Bar user={user} handleSignOut={handleSignOut}/>
        <Content user={user} handleSignOut={handleSignOut}/>
    </>
    
  );
}

export default MainWebsite;