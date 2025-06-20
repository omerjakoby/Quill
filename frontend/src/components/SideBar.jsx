import { Link } from 'react-router-dom';


function SideBar() {
  return (
    <div>
      <h2>Sidebar</h2>
      <ul>
        <li><Link to="/Demo">compose</Link></li>
        <li><button>inbox</button></li>
        <li><button>sent</button></li>
        <li><button>drafts</button></li>
      </ul>
    </div>
  );
}   

export default SideBar;