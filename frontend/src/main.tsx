import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '/index.css'
import App from './App.js'
import { BrowserRouter } from 'react-router-dom'; 


const rootElement = document.getElementById('root');
if (rootElement) {
  createRoot(rootElement).render(
    <StrictMode>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </StrictMode>,
  );
} else {
  throw new Error("Root element not found");
}
