import '../css/UpperBar.css';
import logoFull from '../assets/logo-full-white.png';

function Bar() {
  return (
    <div className="block">
        <div class="quill">Quill</div>
      <div className="bar">
        <a href="#home" className="barherf">Mail</a>
        <a href="#about" className="barherf">Contacts</a>
        <a href="#contact" className="barherf">Calander</a>
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